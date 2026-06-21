import { NavLink, Outlet } from "react-router-dom";
import { clsx } from "clsx";
import { useEffect, useState } from "react";

const nav = [
  { to: "/", label: "Dashboard", end: true },
  { to: "/tags", label: "Tags" },
  { to: "/scenarios", label: "Scenarios" },
  { to: "/traffic", label: "Traffic" },
  { to: "/gateway", label: "Gateway" },
  { to: "/settings", label: "Settings" },
];

export default function Layout() {
  const [dark, setDark] = useState(() => !document.documentElement.classList.contains("light"));

  useEffect(() => {
    document.documentElement.classList.toggle("dark", dark);
    document.documentElement.classList.toggle("light", !dark);
  }, [dark]);

  return (
    <div className="min-h-screen">
      <header className="sticky top-0 z-10 border-b bg-background/80 backdrop-blur">
        <div className="mx-auto flex max-w-6xl items-center gap-6 px-4 py-3">
          <div className="flex items-center gap-2 font-semibold">
            <span className="text-lg tracking-tight">
              <span className="text-primary">◢</span> Cylon
            </span>
            <span className="hidden text-xs text-muted-foreground sm:inline">LoRaWAN simulator</span>
          </div>
          <nav className="flex flex-1 gap-1">
            {nav.map((n) => (
              <NavLink
                key={n.to}
                to={n.to}
                end={n.end}
                className={({ isActive }) =>
                  clsx(
                    "rounded-md px-3 py-1.5 text-sm font-medium transition",
                    isActive ? "bg-muted text-foreground" : "text-muted-foreground hover:text-foreground",
                  )
                }
              >
                {n.label}
              </NavLink>
            ))}
          </nav>
          <button
            onClick={() => setDark((d) => !d)}
            className="rounded-md border px-2 py-1 text-sm hover:bg-muted"
            title="Toggle theme"
          >
            {dark ? "☾" : "☀"}
          </button>
        </div>
      </header>
      <main className="mx-auto max-w-6xl px-4 py-6">
        <Outlet />
      </main>
    </div>
  );
}
