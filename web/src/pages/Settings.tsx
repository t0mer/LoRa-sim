import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { Card, CardTitle, Mono } from "@/components/ui";

interface Health {
  status: string;
  version: string;
  eui: string;
}

function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between border-b border-border/50 py-2 text-sm">
      <span className="text-muted-foreground">{label}</span>
      <span>{value}</span>
    </div>
  );
}

export default function Settings() {
  const health = useQuery<Health>({
    queryKey: ["health"],
    queryFn: () => fetch("/healthz").then((r) => r.json()),
  });
  const gateway = useQuery({ queryKey: ["gateway"], queryFn: api.getGateway });

  return (
    <div className="grid gap-6 lg:grid-cols-2">
      <Card>
        <CardTitle>Application</CardTitle>
        <div className="mt-2">
          <Row label="Version" value={<Mono>{health.data?.version ?? "…"}</Mono>} />
          <Row label="Gateway EUI" value={<Mono>{health.data?.eui ?? "…"}</Mono>} />
          <Row label="Region" value={gateway.data?.region ?? "…"} />
          <Row label="Connection mode" value={gateway.data?.connection_mode ?? "…"} />
          <Row label="Health" value={health.data?.status ?? "…"} />
        </div>
      </Card>

      <Card>
        <CardTitle>Appearance & data</CardTitle>
        <div className="mt-2">
          <Row label="Theme" value="Toggle with the ☾/☀ button in the navbar" />
          <Row label="Event retention" value="Configured server-side (store.events_retention)" />
        </div>
        <p className="mt-4 text-xs text-muted-foreground">
          Cylon is a LoRaWAN simulator. All state lives in SQLite; the UI is a
          client of the documented <Mono>/api</Mono> endpoints.
        </p>
      </Card>
    </div>
  );
}
