import Image from "next/image";

/** A real app screen in a plain phone outline. */
export function PhoneFrame({
  src,
  alt,
  eager = false,
}: {
  src: string;
  alt: string;
  /** For the screen above the fold: loaded straight away, at high priority. */
  eager?: boolean;
}) {
  return (
    <div className="w-full max-w-[300px] rounded-[44px] border border-cloud bg-snow p-2.5">
      <div className="overflow-hidden rounded-[36px] border border-cloud bg-paper">
        <Image
          src={src}
          alt={alt}
          width={400}
          height={860}
          loading={eager ? "eager" : "lazy"}
          fetchPriority={eager ? "high" : "auto"}
          sizes="(min-width: 768px) 300px, 80vw"
          className="h-auto w-full"
        />
      </div>
    </div>
  );
}
