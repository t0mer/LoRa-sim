import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { Button, Card, CardTitle, Input, Mono } from "@/components/ui";

export default function Scenarios() {
  const qc = useQueryClient();
  const tags = useQuery({ queryKey: ["tags"], queryFn: api.listTags, refetchInterval: 3000 });
  const [result, setResult] = useState<string | null>(null);
  const [burst, setBurst] = useState({ count: 100, at_once: 16 });

  const invalidate = () => qc.invalidateQueries({ queryKey: ["tags"] });
  const onResult = (label: string) => (r: Record<string, unknown>) => {
    setResult(`${label}: ${JSON.stringify(r)}`);
    invalidate();
  };
  const onErr = (e: unknown) => setResult(`error: ${e instanceof Error ? e.message : String(e)}`);

  const joinAll = useMutation({ mutationFn: () => api.runScenario("join_all"), onSuccess: onResult("join_all"), onError: onErr });
  const runBurst = useMutation({
    mutationFn: () => api.runScenario("burst", burst),
    onSuccess: onResult("burst"),
    onError: onErr,
  });

  const total = tags.data?.length ?? 0;
  const running = tags.data?.filter((t) => t.running).length ?? 0;
  const joined = tags.data?.filter((t) => t.session?.joined).length ?? 0;

  return (
    <div className="space-y-6">
      <div className="grid grid-cols-3 gap-4">
        <Card><CardTitle>Total tags</CardTitle><div className="mt-1 text-2xl font-semibold">{total}</div></Card>
        <Card><CardTitle>Running</CardTitle><div className="mt-1 text-2xl font-semibold">{running}</div></Card>
        <Card><CardTitle>Joined</CardTitle><div className="mt-1 text-2xl font-semibold">{joined}</div></Card>
      </div>

      <Card>
        <CardTitle>Join all</CardTitle>
        <p className="mt-1 text-sm text-muted-foreground">Start every enabled tag and perform OTAA join over the gateway.</p>
        <Button className="mt-3" variant="primary" onClick={() => joinAll.mutate()} disabled={joinAll.isPending}>
          Run join_all
        </Button>
      </Card>

      <Card>
        <CardTitle>Burst</CardTitle>
        <p className="mt-1 text-sm text-muted-foreground">Send a burst of uplinks across the running fleet.</p>
        <div className="mt-3 flex flex-wrap items-end gap-3">
          <label className="text-xs text-muted-foreground">
            Count
            <Input type="number" className="w-28" value={burst.count} onChange={(e) => setBurst({ ...burst, count: +e.target.value })} />
          </label>
          <label className="text-xs text-muted-foreground">
            Concurrency
            <Input type="number" className="w-28" value={burst.at_once} onChange={(e) => setBurst({ ...burst, at_once: +e.target.value })} />
          </label>
          <Button variant="primary" onClick={() => runBurst.mutate()} disabled={runBurst.isPending || running === 0}>
            Run burst
          </Button>
        </div>
      </Card>

      {result && (
        <Card>
          <CardTitle>Result</CardTitle>
          <Mono className="mt-2 block text-muted-foreground">{result}</Mono>
        </Card>
      )}
    </div>
  );
}
