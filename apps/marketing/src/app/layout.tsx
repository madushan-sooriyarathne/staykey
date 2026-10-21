import type { Metadata } from "next";
import { DM_Sans } from "next/font/google";
import { SITE_URL } from "@/lib/company";
import { appStoreId } from "@/lib/stores";
import "./globals.css";

const dmSans = DM_Sans({
  variable: "--font-dm-sans",
  subsets: ["latin"],
});

const title = "StayKey | Direct bookings for villas in Sri Lanka";
const description =
  "Your own booking page and website widget in 10 minutes. Every booking in one calendar, with no commission on direct stays.";

export function generateMetadata(): Metadata {
  const appId = appStoreId();
  return {
    metadataBase: new URL(SITE_URL),
    title: { default: title, template: "%s | StayKey" },
    description,
    applicationName: "StayKey",
    openGraph: {
      title,
      description,
      url: SITE_URL,
      siteName: "StayKey",
      locale: "en_LK",
      type: "website",
    },
    twitter: { card: "summary_large_image", title, description },
    ...(appId ? { itunes: { appId } } : {}),
  };
}

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="en" className={`${dmSans.variable} h-full`}>
      <body className="min-h-full">{children}</body>
    </html>
  );
}
