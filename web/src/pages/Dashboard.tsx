import { useQuery } from "@tanstack/react-query";
import { BarChart, Bar, XAxis, YAxis, ResponsiveContainer, Tooltip } from "recharts";
import { api } from "@/lib/api";
import { useEventFeed } from "@/lib/ws";
import { Card, CardTitle, Dot, Mono } from "@/components/ui";
import EventsTable from "@/components/EventsTable";

function Stat({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <Card>
      <CardTitle>{label}</CardTitle>
      <div className="mt-1 text-2xl font-semibold">{value}</div>
    </Card>
  );
}

export default function Dashboard() {
  const gateway = useQuery({ queryKey: ["gateway"], queryFn: api.getGateway, refetchInterval: 5000 });
  const seed = useQuery({ queryKey: ["events"], queryFn: () => api.listEvents(200) });
  const events = useEventFeed(seed.data ?? []);

  const ups = events.filter((e) => e.direction === "up" && e.kind === "data").length;
  const downs = events.filter((e) => e.direction === "down").length;
  const joins = events.filter((e) => e.kind === "join").length;

  const chart = [
    { name: "Joins", count: joins },
    { name: "Uplinks", count: ups },
    { name: "Downlinks", count: downs },
  ];

  const g = gateway.data;
  return (
    <div className="space-y-6">
      <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
        <Card>
          <CardTitle>Gateway</CardTitle>
          <div className="mt-1 flex items-center gap-2 text-lg font-semibold">
            <Dot on={g?.status === "connected"} />
            {g?.status ?? "…"}
          </div>
          <Mono className="text-muted-foreground">{g?.eui}</Mono>
        </Card>
        <Stat label="Tag connections" value={g?.tag_conns ?? "…"} />
        <Stat label="WS clients" value={g?.ws_clients ?? "…"} />
        <Stat label="Region" value={g?.region ?? "…"} />
      </div>

      <div className="grid gap-6 lg:grid-cols-3">
        <Card className="lg:col-span-1">
          <CardTitle>Activity</CardTitle>
          <div className="mt-3 h-48">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={chart}>
                <XAxis dataKey="name" stroke="hsl(var(--muted-foreground))" fontSize={12} />
                <YAxis stroke="hsl(var(--muted-foreground))" fontSize={12} allowDecimals={false} />
                <Tooltip
                  contentStyle={{ background: "hsl(var(--card))", border: "1px solid hsl(var(--border))", borderRadius: 8 }}
                />
                <Bar dataKey="count" fill="hsl(var(--primary))" radius={[4, 4, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          </div>
        </Card>

        <Card className="lg:col-span-2">
          <div className="mb-2 flex items-center justify-between">
            <CardTitle>Live traffic</CardTitle>
            <Mono className="text-muted-foreground">{events.length} events</Mono>
          </div>
          <div className="max-h-72 overflow-y-auto">
            <EventsTable events={events.slice(0, 50)} />
          </div>
        </Card>
      </div>
    </div>
  );
}
