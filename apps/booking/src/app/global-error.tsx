"use client";

// A global error boundary replaces the root layout, so it is a client component with its own
// <html> and reports the error itself.
import { useEffect } from "react";

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
        <main className="mx-auto flex min-h-screen max-w-xl flex-col justify-center gap-3 px-6">
          <h1 className="font-semibold text-3xl text-obsidian">This page didn't load</h1>
          <p className="text-steel">
            Something went wrong on our side. Please try again in a moment, or contact the property
            directly.
          </p>
          <button
            type="button"
            onClick={retry}
            className="mt-2 self-start rounded-full bg-obsidian px-5 py-3 font-medium text-paper focus-visible:outline-2 focus-visible:outline-offset-2"
          >
            Try again
          </button>
        </main>
      </body>
    </html>
  );
}
