"use client";

// A global error boundary replaces the root layout, so it is a client component with its own
// <html> and reports the error itself.
import { useEffect } from "react";

import { SUPPORT } from "@/lib/company";
import "./globals.css";

export default function GlobalError({
  error,
  retry,
}: {
  error: Error & { digest?: string };
  retry: () => void;
}) {
  useEffect(() => {
    import("@sentry/nextjs").then((Sentry) => Sentry.captureException(error));
  }, [error]);

  return (
    <html lang="en" className="h-full">
      <body className="min-h-full">
        <title>Something went wrong | StayKey</title>
        <main className="mx-auto flex max-w-3xl flex-col gap-5 px-5 py-24 md:px-6">
          <h1 className="font-semibold text-4xl text-obsidian md:text-5xl">
            This page didn't load
          </h1>
          <p className="text-body-lg text-steel">
            Something went wrong on our side. Please try again in a moment, or write to us at{" "}
            <a className="focus-ring text-iron underline" href={`mailto:${SUPPORT.email}`}>
              {SUPPORT.email}
            </a>
            .
          </p>
          <button
            type="button"
            onClick={retry}
            className="focus-ring self-start rounded-button bg-obsidian px-4 py-3 text-sm text-snow shadow-button"
          >
            Try again
          </button>
        </main>
      </body>
    </html>
  );
}
