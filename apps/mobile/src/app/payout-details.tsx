import { router } from "expo-router";
import { useState } from "react";
import { View } from "react-native";
import { useCardPayments } from "@/api/cards";
import { messageFor } from "@/api/errors";
import { useSetPayoutProfile } from "@/api/payouts";
import { Button, Checkbox, Field, InfoNote } from "@/components/controls";
import { I } from "@/components/icons";
import { Page, SectionHeader } from "@/components/kit";
import { useCan } from "@/data/hooks";
import { isValidAccountNumber, isValidNIC, PAYOUT_TERMS_VERSION } from "@/features/payouts/payouts";
import { haptics } from "@/lib/haptics";

/**
 * Where StayKey pays out card money: the owner's legal name, NIC and a bank account in their own
 * name. The NIC and account number are entered again on every change, since the server only
 * shows their last four. Owners only.
 */
export default function PayoutDetailsScreen() {
  const can = useCan();
  const owner = can("billing");
  const cards = useCardPayments({ enabled: owner }).data;
  const saved = cards?.payoutProfile;
  const save = useSetPayoutProfile();
  const [legalName, setLegalName] = useState(saved?.legalName ?? "");
  const [nic, setNic] = useState("");
  const [bankName, setBankName] = useState(saved?.bankName ?? "");
  const [branch, setBranch] = useState(saved?.branch ?? "");
  const [accountName, setAccountName] = useState(saved?.accountName ?? "");
  const [accountNumber, setAccountNumber] = useState("");
  const [accepted, setAccepted] = useState(saved?.termsVersion === PAYOUT_TERMS_VERSION);
  const [touched, setTouched] = useState(false);
  const [problem, setProblem] = useState<string | null>(null);

  if (!owner) {
    return (
      <Page title="Payout details">
        <InfoNote icon={I.lock}>Only the account owner can set where payouts go.</InfoNote>
      </Page>
    );
  }

  const errors = {
    legalName: legalName.trim().length < 2 ? "Enter your name as on your NIC" : null,
    nic: isValidNIC(nic) ? null : "Enter a NIC number, 9 digits and V or X, or 12 digits",
    bankName: bankName.trim().length < 2 ? "Enter your bank" : null,
    branch: branch.trim().length < 2 ? "Enter the branch" : null,
    accountName: accountName.trim().length < 2 ? "Enter the name on the account" : null,
    accountNumber: isValidAccountNumber(accountNumber)
      ? null
      : "Enter the account number, digits only",
  };
  const valid = Object.values(errors).every((e) => e === null) && accepted;
  const show = (e: string | null) => (touched ? e : null);

  const submit = () => {
    setTouched(true);
    setProblem(null);
    if (!valid) return;
    save.mutate(
      {
        legalName,
        nic,
        bankName,
        branch,
        accountName,
        accountNumber,
        acceptTerms: PAYOUT_TERMS_VERSION,
      },
      {
        onSuccess: () => {
          haptics.success();
          router.back();
        },
        onError: (e) => setProblem(messageFor(e)),
      },
    );
  };

  return (
    <Page
      title="Payout details"
      footer={<Button title="Save" loading={save.isPending} onPress={submit} />}
    >
      <InfoNote icon={I.lock}>
        StayKey keeps your NIC and account number encrypted and only shows their last four digits.
        {saved ? " Enter them again to save a change." : ""}
      </InfoNote>

      <SectionHeader title="You" />
      <View style={{ gap: 12 }}>
        <Field
          testID="payout-legal-name"
          label="Name as on your NIC"
          value={legalName}
          onChangeText={setLegalName}
          autoCapitalize="words"
          error={show(errors.legalName)}
        />
        <Field
          testID="payout-nic"
          label="NIC number"
          placeholder={saved ? `Ending ${saved.nicLast4}` : "199012345678"}
          value={nic}
          onChangeText={setNic}
          autoCapitalize="characters"
          autoCorrect={false}
          error={show(errors.nic)}
        />
      </View>

      <SectionHeader title="Bank account in your own name" />
      <View style={{ gap: 12 }}>
        <Field
          testID="payout-bank"
          label="Bank"
          placeholder="Commercial Bank"
          value={bankName}
          onChangeText={setBankName}
          error={show(errors.bankName)}
        />
        <Field
          testID="payout-branch"
          label="Branch"
          placeholder="Galle"
          value={branch}
          onChangeText={setBranch}
          error={show(errors.branch)}
        />
        <Field
          testID="payout-account-name"
          label="Name on the account"
          value={accountName}
          onChangeText={setAccountName}
          autoCapitalize="words"
          error={show(errors.accountName)}
        />
        <Field
          testID="payout-account-number"
          label="Account number"
          placeholder={saved ? `Ending ${saved.accountLast4}` : ""}
          value={accountNumber}
          onChangeText={setAccountNumber}
          keyboardType="number-pad"
          error={show(errors.accountNumber)}
        />
      </View>
      {saved ? (
        <InfoNote icon={I.clock}>
          A new bank, branch or account number holds your next payout for 72 hours, and we email you
          about it.
        </InfoNote>
      ) : null}

      <SectionHeader title="Payout terms" />
      <InfoNote icon={I.doc}>
        StayKey takes your guests' card payments and pays you every other Monday by bank transfer,
        for stays that have started, less a 3% processing charge. Payouts under Rs 1,000 wait for
        the next one. Refunds come out of your balance, and the charge isn't returned on a refund.
      </InfoNote>
      <Checkbox
        checked={accepted}
        onChange={setAccepted}
        label={`I accept the payout terms (${PAYOUT_TERMS_VERSION}) and the account is in my own name`}
      />
      {touched && !accepted ? (
        <InfoNote icon={I.warning}>Accept the payout terms to save.</InfoNote>
      ) : null}
      {problem ? <InfoNote icon={I.warning}>{problem}</InfoNote> : null}
    </Page>
  );
}
