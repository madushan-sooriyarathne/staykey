// Command payouts is how StayKey's team pays owners on StayKey payouts. The API prepares a
// batch every other Monday; this command lists it, writes the bank's bulk upload file, and
// records what happened. Run it with the API's environment (DATABASE_URL and STAYKEY_DATA_KEY),
// as a person, never from a script.
//
// Usage:
//
//	go run ./cmd/payouts list
//	go run ./cmd/payouts prepare [-at 2026-11-02T08:00:00+05:30]
//	go run ./cmd/payouts export <batch> > batch.csv
//	go run ./cmd/payouts paid <batch> -ref <bank reference>
//	go run ./cmd/payouts failed <payout> -reason "Account closed"
//	go run ./cmd/payouts adjust <account> <rupees> -reason "Chargeback on KV-2041"
//
// The API prepares each batch on its own; prepare is for a batch it missed, and -at, refused in
// production, tries a payout Monday on a development database.
//
// The routine: list, export, check the file by eye, upload it to the bank, then mark the batch
// paid. A payout export marks as held (bank details changed after it was prepared) is left out
// of the file; fail it, and its money waits for the next payout. A transfer the bank returns
// later is failed the same way.
package main

import (
	"context"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/google/uuid"

	"staykey.direct/api/internal/config"
	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/secret"
	"staykey.direct/api/internal/store"
)

const usage = `usage:
  payouts list
  payouts prepare [-at <RFC 3339 time>]
  payouts export <batch>
  payouts paid <batch> -ref <bank reference>
  payouts failed <payout> -reason <why>
  payouts adjust <account> <rupees> -reason <why>`

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "payouts:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, out, info io.Writer) error {
	if len(args) == 0 {
		return errors.New(usage)
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	box, err := secret.New(cfg.DataKey)
	if err != nil {
		return err
	}
	st, err := store.NewPostgres(ctx, cfg.DatabaseURL, store.WithSecrets(box))
	if err != nil {
		return err
	}
	defer st.Close()

	cmd, rest := args[0], args[1:]
	switch cmd {
	case "list":
		return list(ctx, st, out)
	case "prepare":
		_, fs, err := idAndFlags(rest, 0, "at")
		if err != nil {
			return err
		}
		at := time.Now()
		if fs["at"] != "" {
			if cfg.IsProduction() {
				return errors.New("-at only works outside production")
			}
			if at, err = time.Parse(time.RFC3339, fs["at"]); err != nil {
				return fmt.Errorf("-at: %w", err)
			}
		}
		b, prepared, err := st.PreparePayouts(ctx, at)
		if err != nil {
			return err
		}
		if !prepared {
			fmt.Fprintln(out, "Nothing to prepare: the latest payout Monday's batch is already prepared, or none has come yet.")
			return nil
		}
		fmt.Fprintf(out, "Prepared batch %s for card payments up to %s: %d payouts, %s in all.\n",
			b.ID, b.PeriodEnd.Format(time.DateOnly), b.Payouts, domain.FormatMoney(b.Total, domain.CardCurrency))
		return nil
	case "export":
		id, _, err := idAndFlags(rest, 1)
		if err != nil {
			return err
		}
		return export(ctx, st, id[0], out, info)
	case "paid":
		ids, fs, err := idAndFlags(rest, 1, "ref")
		if err != nil {
			return err
		}
		ref := strings.TrimSpace(fs["ref"])
		if ref == "" || len(ref) > 100 {
			return errors.New("paid needs -ref with the bank's reference for the batch, up to 100 characters")
		}
		n, err := st.MarkBatchPaid(ctx, ids[0], ref, time.Now())
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "Marked %d payouts paid; owners will get an email and an Activity line.\n", n)
		return nil
	case "failed":
		ids, fs, err := idAndFlags(rest, 1, "reason")
		if err != nil {
			return err
		}
		reason := strings.TrimSpace(fs["reason"])
		if reason == "" {
			return errors.New("failed needs -reason")
		}
		amount, err := st.FailPayout(ctx, ids[0], reason, time.Now())
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "Payout failed; %s is back in the owner's balance.\n", domain.FormatMoney(amount, domain.CardCurrency))
		return nil
	case "adjust":
		if len(rest) < 2 {
			return errors.New(usage)
		}
		account, err := uuid.Parse(rest[0])
		if err != nil {
			return fmt.Errorf("account: %w", err)
		}
		amount, err := parseRupees(rest[1])
		if err != nil {
			return err
		}
		_, fs, err := idAndFlags(append([]string{rest[0]}, rest[2:]...), 1, "reason")
		if err != nil {
			return err
		}
		if err := st.AdjustPayouts(ctx, account, amount, strings.TrimSpace(fs["reason"]), time.Now()); err != nil {
			return err
		}
		fmt.Fprintf(out, "Adjusted by %s.\n", domain.FormatMoney(amount, domain.CardCurrency))
		return nil
	}
	return errors.New(usage)
}

// idAndFlags reads n ids, then the named string flags.
func idAndFlags(args []string, n int, flags ...string) ([]uuid.UUID, map[string]string, error) {
	if len(args) < n {
		return nil, nil, errors.New(usage)
	}
	ids := make([]uuid.UUID, n)
	for i := range n {
		id, err := uuid.Parse(args[i])
		if err != nil {
			return nil, nil, fmt.Errorf("%q isn't an id: %w", args[i], err)
		}
		ids[i] = id
	}
	fs := flag.NewFlagSet("payouts", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	vals := make(map[string]*string, len(flags))
	for _, f := range flags {
		vals[f] = fs.String(f, "", "")
	}
	if err := fs.Parse(args[n:]); err != nil {
		return nil, nil, fmt.Errorf("%w\n%s", err, usage)
	}
	if fs.NArg() > 0 {
		return nil, nil, fmt.Errorf("unexpected %q\n%s", fs.Arg(0), usage)
	}
	out := make(map[string]string, len(flags))
	for f, v := range vals {
		out[f] = *v
	}
	return ids, out, nil
}

// parseRupees reads an amount like 1500, -1500.5 or 1,500.00 into rupee cents.
func parseRupees(s string) (int64, error) {
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", "")
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	whole, frac, _ := strings.Cut(s, ".")
	if len(frac) > 2 {
		return 0, fmt.Errorf("%q has more than two decimals", s)
	}
	frac += strings.Repeat("0", 2-len(frac))
	w, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%q isn't an amount in rupees", s)
	}
	f, err := strconv.ParseInt(frac, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%q isn't an amount in rupees", s)
	}
	cents := w*100 + f
	if neg {
		cents = -cents
	}
	return cents, nil
}

func list(ctx context.Context, st *store.Postgres, out io.Writer) error {
	batches, err := st.PayoutBatches(ctx, 20)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "BATCH\tPERIOD END\tPAYOUTS\tTOTAL\tPAID\tBANK REF")
	for _, b := range batches {
		paid := "not yet"
		if b.PaidAt != nil {
			paid = b.PaidAt.Format(time.DateOnly)
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\t%s\n", b.ID, b.PeriodEnd.Format(time.DateOnly), b.Payouts,
			domain.FormatMoney(b.Total, domain.CardCurrency), paid, b.BankRef)
	}
	return w.Flush()
}

// export writes the batch's prepared payouts as the bank's bulk upload: name, bank, branch,
// account, amount in rupees, and a reference the owner sees on their statement.
func export(ctx context.Context, st *store.Postgres, batchID uuid.UUID, out, info io.Writer) error {
	transfers, err := st.BatchTransfers(ctx, batchID)
	if err != nil {
		return err
	}
	w := csv.NewWriter(out)
	if err := w.Write([]string{"name", "bank", "branch", "account", "amount", "reference"}); err != nil {
		return err
	}
	var total int64
	rows := 0
	for _, t := range transfers {
		if t.Status != domain.PayoutPrepared {
			continue
		}
		if t.Held {
			fmt.Fprintf(info, "held: payout %s for account %s, bank details changed after it was prepared; fail it\n", t.PayoutID, t.AccountID)
			continue
		}
		amount := fmt.Sprintf("%d.%02d", t.Amount/100, t.Amount%100)
		// Payout ids start with their time, so the random tail tells a batch's payouts apart.
		id := t.PayoutID.String()
		ref := "STAYKEY " + strings.ToUpper(id[len(id)-8:])
		if err := w.Write([]string{t.AccountName, t.BankName, t.Branch, t.AccountNumber, amount, ref}); err != nil {
			return err
		}
		total += t.Amount
		rows++
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return err
	}
	fmt.Fprintf(info, "%d transfers, %s in all\n", rows, domain.FormatMoney(total, domain.CardCurrency))
	return nil
}
