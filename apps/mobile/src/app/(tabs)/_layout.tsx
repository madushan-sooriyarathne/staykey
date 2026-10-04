import { colors } from "@staykey/tokens";
import { Tabs } from "expo-router";
import { SymbolView, type SymbolViewProps } from "expo-symbols";
import type { ColorValue } from "react-native";
import { font } from "@/components/ui";

type IconName = SymbolViewProps["name"];

function icon(name: IconName) {
  return ({ color }: { color: ColorValue }) => (
    <SymbolView name={name} tintColor={color} size={24} />
  );
}

export default function TabLayout() {
  return (
    <Tabs
      screenOptions={{
        headerShown: false,
        tabBarActiveTintColor: colors.obsidian,
        tabBarInactiveTintColor: colors.ash,
        tabBarLabelStyle: { fontFamily: font.medium, fontSize: 12 },
        tabBarStyle: { backgroundColor: colors.snow, borderTopColor: colors.cloud },
      }}
    >
      <Tabs.Screen
        name="index"
        options={{
          title: "Today",
          tabBarIcon: icon({ ios: "sun.max", android: "light_mode", web: "light_mode" }),
        }}
      />
      <Tabs.Screen
        name="calendar"
        options={{
          title: "Calendar",
          tabBarIcon: icon({ ios: "calendar", android: "calendar_month", web: "calendar_month" }),
        }}
      />
      <Tabs.Screen
        name="bookings"
        options={{
          title: "Bookings",
          tabBarIcon: icon({ ios: "list.bullet", android: "list", web: "list" }),
        }}
      />
      <Tabs.Screen
        name="properties"
        options={{
          title: "Properties",
          tabBarIcon: icon({ ios: "house", android: "home", web: "home" }),
        }}
      />
      <Tabs.Screen
        name="more"
        options={{
          title: "More",
          tabBarIcon: icon({ ios: "square.grid.2x2", android: "grid_view", web: "grid_view" }),
        }}
      />
    </Tabs>
  );
}
