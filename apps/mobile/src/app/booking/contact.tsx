import { useLocalSearchParams } from "expo-router";
import { View } from "react-native";
import { Pill } from "@/components/controls";
import { I } from "@/components/icons";
import { Avatar, List, ListRow, SectionHeader, SheetPage } from "@/components/kit";
import { useBooking, useProperty } from "@/data/hooks";
import { useData } from "@/data/store";
import { call, email, fillTemplate, openWhatsApp } from "@/lib/contact";

/** Reach the guest by WhatsApp, call or email, with prefilled templates. */
export default function ContactGuest() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const booking = useBooking(id);
  const property = useProperty(booking?.propertyId);
  const templates = useData((s) => s.templates).filter((t) => t.channels.includes("whatsapp"));
  if (!booking || !property) return null;
  const { name, phone, email: address } = booking.guest;
  const first = name.split(" ")[0];

  return (
    <SheetPage title={name} subtitle={phone ?? address}>
      <List>
        <ListRow
          leading={<Avatar name={name} />}
          title={name}
          subtitle={phone ?? "No phone number"}
          chevron={false}
        />
      </List>
      <List>
        {phone ? (
          <ListRow
            icon={I.chat}
            title="WhatsApp"
            subtitle={`Open a chat with ${first}`}
            onPress={() => openWhatsApp(phone)}
          />
        ) : null}
        {phone ? (
          <ListRow icon={I.phone} title="Call" subtitle={phone} onPress={() => call(phone)} />
        ) : null}
        {address ? (
          <ListRow
            icon={I.mail}
            title="Email"
            subtitle={address}
            onPress={() => email(address, `Your stay at ${property.name}`)}
          />
        ) : null}
      </List>
      {phone && templates.length ? (
        <View style={{ gap: 8 }}>
          <SectionHeader title="Send a template on WhatsApp" />
          <View style={{ flexDirection: "row", flexWrap: "wrap", gap: 8 }}>
            {templates.map((t) => (
              <Pill
                key={t.id}
                label={t.name}
                selected={false}
                onPress={() => openWhatsApp(phone, fillTemplate(t.body, booking, property))}
              />
            ))}
          </View>
        </View>
      ) : null}
    </SheetPage>
  );
}
