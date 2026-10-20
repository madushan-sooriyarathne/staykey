import type { CardMode } from "@staykey/api-client";
import { useState } from "react";
import { Linking, View } from "react-native";
import Animated, { FadeInDown, FadeOut } from "react-native-reanimated";
import { useCardPayments, useConnectCards, useDisconnectCards, useSetCardMode } from "@/api/cards";
import { messageFor } from "@/api/errors";
import { Tag } from "@/components/brand";
import { Button, ChoiceCard, Field, InfoNote } from "@/components/controls";
import { I } from "@/components/icons";
import { Card, KV, Page, SectionHeader } from "@/components/kit";
import { useCan } from "@/data/hooks";
import { haptics } from "@/lib/haptics";

/**
 * Where guests' card payments go: the owner's own Payments.lk account, or StayKey's account with
 * a payout every two weeks, when the server offers it. Owners only.
 */
export default function CardPaymentsScreen() {
  const can = useCan();
  const owner = can("billing");
  const query = useCardPayments({ enabled: owner });
  const setMode = useSetCardMode();
  const connect = useConnectCards();
  const disconnect = useDisconnectCards();
  const [key, setKey] = useState("");
  const [problem, setProblem] = useState<string | null>(null);
  const cards = query.data;

  if (!owner) {
    return (
      <Page title="Card payments">
        <InfoNote icon={I.lock}>Only the account owner can set up card payments.</InfoNote>
      </Page>
    );
  }
  if (!cards) {
    return (
      <Page title="Card payments">
        {query.error ? <InfoNote icon={I.warning}>{messageFor(query.error)}</InfoNote> : null}
      </Page>
    );
  }

  const choose = (mode: CardMode) => {
    setProblem(null);
    setMode.mutate(mode, { onError: (e) => setProblem(messageFor(e)) });
  };
  const save = () => {
    setProblem(null);
    connect.mutate(key.trim(), {
      onSuccess: () => {
        haptics.success();
        setKey("");
      },
      onError: (e) => setProblem(messageFor(e)),
    });
  };
  const conn = cards.connection;
  const own = cards.mode === "own" || (cards.mode === "off" && !cards.payoutsAvailable);

  return (
    <Page title="Card payments">
      <InfoNote icon={I.card}>
        Guests pay by card on Payments.lk's secure page, in rupees. Turn cards on for each property
        in its payment methods.
      </InfoNote>

      <SectionHeader title="Where card payments go" />
      <View style={{ gap: 10 }}>
        <ChoiceCard
          testID="card-mode-own"
          icon={I.bank}
          title="Your own Payments.lk account"
          description="Money goes straight to you. Payments.lk needs a business registered in Sri Lanka. No StayKey charge."
          selected={cards.mode === "own"}
          onPress={() => choose("own")}
        />
        {cards.payoutsAvailable ? (
          <ChoiceCard
            testID="card-mode-payouts"
            icon={I.cash}
            title="StayKey payouts"
            description="StayKey takes the payment and pays you by bank transfer every two weeks, less a 3% processing charge."
            selected={cards.mode === "payouts"}
            onPress={() => choose("payouts")}
          />
        ) : null}
      </View>

      {own ? (
        <Animated.View
          entering={FadeInDown.duration(220)}
          exiting={FadeOut.duration(120)}
          style={{ gap: 12 }}
        >
          <SectionHeader title="Your Payments.lk account" />
          {conn ? (
            <Card>
              <KV
                label="Status"
                value={
                  <Tag
                    tone={conn.status === "connected" ? "spark" : "ember"}
                    label={conn.status === "connected" ? "Connected" : "Key not accepted"}
                  />
                }
              />
              <KV label="Mode" value={conn.mode === "live" ? "Live" : "Test, no real charges"} />
              <KV label="Key" value={`Ending ${conn.last4}`} />
              <View style={{ paddingTop: 8 }}>
                <Button
                  compact
                  variant="ghost"
                  title="Disconnect"
                  icon={I.trash}
                  loading={disconnect.isPending}
                  onPress={() =>
                    disconnect.mutate(undefined, { onError: (e) => setProblem(messageFor(e)) })
                  }
                />
              </View>
            </Card>
          ) : (
            <InfoNote icon={I.key}>
              Sign up at payments.lk, then in its dashboard open Developers and create a secret key.
              Paste it here; StayKey keeps it encrypted and only shows its last four characters.
            </InfoNote>
          )}
          {conn?.status !== "connected" ? (
            <View style={{ gap: 10 }}>
              <Field
                testID="card-key"
                label="Secret key"
                placeholder="sk_live_..."
                value={key}
                onChangeText={setKey}
                autoCapitalize="none"
                autoCorrect={false}
                secureTextEntry
              />
              <Button
                title="Connect"
                disabled={key.trim().length < 16}
                loading={connect.isPending}
                onPress={save}
              />
              <Button
                compact
                variant="ghost"
                title="Open payments.lk"
                icon={I.globe}
                onPress={() => Linking.openURL("https://payments.lk")}
              />
            </View>
          ) : null}
        </Animated.View>
      ) : null}

      {problem ? <InfoNote icon={I.warning}>{problem}</InfoNote> : null}
    </Page>
  );
}
