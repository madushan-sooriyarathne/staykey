const benefits = [
  {
    title: "Your own booking page",
    body: "Live at yourvilla.staykey.direct the day you sign up. Share it on WhatsApp, Instagram and Google.",
  },
  {
    title: "One calendar for everything",
    body: "Direct bookings, WhatsApp bookings and your Airbnb and Booking.com calendars, side by side.",
  },
  {
    title: "No commission on direct stays",
    body: "Guests pay you by bank transfer, on arrival or by card. StayKey never takes a cut.",
  },
];

const problems = [
  "Bookings scattered across WhatsApp, Excel and OTA inboxes",
  "Commission on guests who would have booked direct",
  "Double bookings when calendars live in different places",
];

export default function Home() {
  return (
    <>
      <header className="mx-auto flex max-w-6xl items-center justify-between px-6 py-5">
        <span className="font-semibold text-lg text-obsidian">StayKey</span>
        <a
          href="#get-started"
          className="rounded-button bg-obsidian px-4 py-3 text-snow text-sm shadow-button"
        >
          Get the app
        </a>
      </header>

      <main>
        <section className="mx-auto grid max-w-6xl gap-10 px-6 pt-16 pb-20 md:grid-cols-[1.4fr_1fr] md:items-end md:pt-24">
          <h1 className="font-semibold text-5xl text-obsidian leading-[1.12] md:text-display">
            Take direct bookings in 10 minutes
          </h1>
          <div className="flex flex-col gap-5">
            <p className="text-steel">
              StayKey gives villa and guesthouse owners in Sri Lanka a booking page, a website
              widget and one calendar, all set up from your phone.
            </p>
            <div className="flex flex-wrap gap-2">
              <span className="rounded-badge bg-ember px-2 py-1 text-caption text-snow">
                Built for Sri Lanka
              </span>
              <span className="rounded-badge border border-cloud bg-snow px-2 py-1 text-caption">
                iOS and Android
              </span>
            </div>
          </div>
        </section>

        <section className="mx-auto grid max-w-6xl gap-4 px-6 md:grid-cols-3">
          {benefits.map((b) => (
            <article key={b.title} className="rounded-card-lg border border-cloud bg-snow p-7">
              <h2 className="font-semibold text-obsidian text-xl">{b.title}</h2>
              <p className="mt-2 text-steel">{b.body}</p>
            </article>
          ))}
        </section>

        <section className="mx-auto max-w-6xl px-6 py-20">
          <div className="relative overflow-hidden rounded-card-lg bg-graphite p-8 text-snow md:p-12">
            <div
              aria-hidden
              className="pointer-events-none absolute -top-24 -right-20 size-72 rounded-full bg-[radial-gradient(circle,rgba(254,69,226,0.5),rgba(255,90,0,0.3)_40%,transparent_70%)]"
            />
            <h2 className="relative font-bold text-3xl md:text-4xl">What StayKey replaces</h2>
            <ul className="relative mt-6 flex flex-col gap-4">
              {problems.map((p) => (
                <li key={p} className="flex gap-3 font-medium text-xl">
                  <span aria-hidden className="text-ember">
                    &rarr;
                  </span>
                  {p}
                </li>
              ))}
            </ul>
          </div>
        </section>

        <section id="get-started" className="mx-auto max-w-6xl px-6 pb-24">
          <h2 className="font-semibold text-4xl text-obsidian">Ready when you are</h2>
          <p className="mt-3 max-w-xl text-steel">
            Download StayKey, add your property and photos, and share your booking link before your
            next guest messages you.
          </p>
        </section>
      </main>

      <footer className="mx-auto max-w-6xl border-cloud border-t px-6 py-8 text-fog text-sm">
        StayKey, staykey.direct
      </footer>
    </>
  );
}
