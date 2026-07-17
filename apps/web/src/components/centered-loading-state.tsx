import { LoaderCircle } from "lucide-react";

export function CenteredLoadingState({
  className = "min-h-40",
  title,
}: {
  className?: string;
  title: string;
}) {
  return (
    <section
      aria-live="polite"
      className={`flex w-full items-center justify-center px-4 py-10 text-center ${className}`}
      role="status"
    >
      <div>
        <LoaderCircle aria-hidden="true" className="mx-auto h-7 w-7 animate-spin text-blue-700" />
        <p className="mt-3 text-sm font-medium text-slate-700">{title}</p>
      </div>
    </section>
  );
}
