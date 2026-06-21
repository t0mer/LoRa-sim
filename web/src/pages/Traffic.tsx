import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { useEventFeed } from "@/lib/ws";
import { Button, Card, CardTitle, Mono } from "@/components/ui";
import EventsTable from "@/components/EventsTable";

export default function Traffic() {
  const seed = useQuery({ queryKey: ["events"], queryFn: () => api.listEvents(200) });
  const events = useEventFeed(seed.data ?? []);
  const [dir, setDir] = useState<"all" | "up" | "down">("all");

  const filtered = dir === "all" ? events : events.filter((e) => e.direction === dir);

  return (
    <Card>
      <div className="mb-3 flex items-center justify-between">
        <CardTitle>Traffic log</CardTitle>
        <div className="flex items-center gap-2">
          <Mono className="text-muted-foreground">{filtered.length} events (live)</Mono>
          {(["all", "up", "down"] as const).map((d) => (
            <Button key={d} variant={dir === d ? "primary" : "ghost"} onClick={() => setDir(d)}>
              {d}
            </Button>
          ))}
        </div>
      </div>
      <div className="max-h-[70vh] overflow-y-auto">
        <EventsTable events={filtered} />
      </div>
    </Card>
  );
}
