import type { Metadata } from "next";
import { DM_Sans } from "next/font/google";
import "./globals.css";

const dmSans = DM_Sans({
  variable: "--font-dm-sans",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  metadataBase: new URL("https://staykey.direct"),
  title: "StayKey | Direct bookings for villas in Sri Lanka",
  description:
    "Your own booking page and website widget in 10 minutes. Every booking in one calendar, with no commission on direct stays.",
  openGraph: {
    title: "StayKey | Direct bookings for villas in Sri Lanka",
    url: "https://staykey.direct",
    siteName: "StayKey",
    type: "website",
  },
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="en" className={`${dmSans.variable} h-full`}>
      <body className="min-h-full">{children}</body>
    </html>
  );
}
