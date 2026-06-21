import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { Button, Card, CardTitle, Dot, Input, Mono } from "@/components/ui";

function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between border-b border-border/50 py-2 text-sm">
      <span className="text-muted-foreground">{label}</span>
      <span>{value}</span>
    </div>
  );
}

export default function GatewayPage() {
  const qc = useQueryClient();
  const gw = useQuery({ queryKey: ["gateway"], queryFn: api.getGateway, refetchInterval: 5000 });
  const [region, setRegion] = useState("");
  const [subBand, setSubBand] = useState(0);

  const update = useMutation({
    mutationFn: () => api.updateGateway({ region: region || gw.data?.region, sub_band: subBand || gw.data?.sub_band }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["gateway"] }),
  });

  const g = gw.data;
  return (
    <div className="grid gap-6 lg:grid-cols-2">
      <Card>
        <CardTitle>Status</CardTitle>
        <div className="mt-2">
          <Row label="EUI" value={<Mono>{g?.eui}</Mono>} />
          <Row label="Status" value={<span className="flex items-center gap-2"><Dot on={g?.status === "connected"} />{g?.status}</span>} />
          <Row label="TCP listener" value={<Mono>{g?.tcp_addr || "—"}</Mono>} />
          <Row label="Region" value={g?.region} />
          <Row label="Sub-band" value={g?.sub_band} />
          <Row label="Connection mode" value={g?.connection_mode} />
          <Row label="Tag connections" value={g?.tag_conns} />
        </div>
      </Card>

      <Card>
        <CardTitle>Configuration</CardTitle>
        <p className="mt-1 text-xs text-muted-foreground">Changes persist to the database (applied on restart).</p>
        <form
          className="mt-3 space-y-3"
          onSubmit={(e) => {
            e.preventDefault();
            update.mutate();
          }}
        >
          <label className="block text-xs text-muted-foreground">
            Region
            <Input placeholder={g?.region} value={region} onChange={(e) => setRegion(e.target.value)} />
          </label>
          <label className="block text-xs text-muted-foreground">
            Sub-band
            <Input type="number" placeholder={String(g?.sub_band ?? "")} value={subBand || ""} onChange={(e) => setSubBand(+e.target.value)} />
          </label>
          <Button type="submit" variant="primary" disabled={update.isPending}>
            Save
          </Button>
          {update.isSuccess && <span className="ml-2 text-sm text-emerald-400">saved</span>}
        </form>
      </Card>
    </div>
  );
}
