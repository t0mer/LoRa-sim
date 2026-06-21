import { clsx } from "clsx";
import type { ButtonHTMLAttributes, HTMLAttributes, InputHTMLAttributes, ReactNode } from "react";

export function Card({ className, ...p }: HTMLAttributes<HTMLDivElement>) {
  return <div className={clsx("rounded-xl border bg-card p-4 shadow-sm", className)} {...p} />;
}

export function CardTitle({ className, ...p }: HTMLAttributes<HTMLHeadingElement>) {
  return <h3 className={clsx("text-sm font-semibold text-muted-foreground", className)} {...p} />;
}

type Variant = "primary" | "ghost" | "danger";
const variants: Record<Variant, string> = {
  primary: "bg-primary text-primary-foreground hover:opacity-90",
  ghost: "border bg-transparent hover:bg-muted",
  danger: "border border-red-500/40 text-red-400 hover:bg-red-500/10",
};

export function Button({
  variant = "ghost",
  className,
  ...p
}: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: Variant }) {
  return (
    <button
      className={clsx(
        "inline-flex items-center justify-center gap-1.5 rounded-md px-3 py-1.5 text-sm font-medium transition disabled:opacity-50",
        variants[variant],
        className,
      )}
      {...p}
    />
  );
}

export function Input({ className, ...p }: InputHTMLAttributes<HTMLInputElement>) {
  return (
    <input
      className={clsx(
        "w-full rounded-md border bg-background px-3 py-1.5 text-sm outline-none focus:border-primary",
        className,
      )}
      {...p}
    />
  );
}

export function Badge({ children, tone = "muted" }: { children: ReactNode; tone?: "muted" | "green" | "blue" | "red" | "violet" }) {
  const tones = {
    muted: "bg-muted text-muted-foreground",
    green: "bg-emerald-500/15 text-emerald-400",
    blue: "bg-blue-500/15 text-blue-400",
    red: "bg-red-500/15 text-red-400",
    violet: "bg-violet-500/15 text-violet-400",
  };
  return <span className={clsx("rounded px-1.5 py-0.5 text-xs font-medium", tones[tone])}>{children}</span>;
}

export function Dot({ on }: { on: boolean }) {
  return (
    <span
      className={clsx("inline-block h-2 w-2 rounded-full", on ? "bg-emerald-400 shadow-[0_0_8px] shadow-emerald-400" : "bg-zinc-600")}
    />
  );
}

export function Mono({ children, className }: { children: ReactNode; className?: string }) {
  return <span className={clsx("font-mono text-xs", className)}>{children}</span>;
}
