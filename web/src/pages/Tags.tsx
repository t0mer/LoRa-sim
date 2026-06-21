import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, type Tag } from "@/lib/api";
import { Badge, Button, Card, CardTitle, Dot, Input, Mono } from "@/components/ui";

export default function Tags() {
  const qc = useQueryClient();
  const tags = useQuery({ queryKey: ["tags"], queryFn: api.listTags, refetchInterval: 4000 });
  const [err, setErr] = useState<string | null>(null);
  const invalidate = () => qc.invalidateQueries({ queryKey: ["tags"] });
  const onErr = (e: unknown) => setErr(e instanceof Error ? e.message : String(e));

  const create = useMutation({ mutationFn: api.createTags, onSuccess: invalidate, onError: onErr });
  const del = useMutation({ mutationFn: api.deleteTag, onSuccess: invalidate, onError: onErr });
  const join = useMutation({ mutationFn: api.joinTag, onSuccess: invalidate, onError: onErr });
  const uplink = useMutation({ mutationFn: (id: number) => api.uplinkTag(id), onSuccess: invalidate, onError: onErr });
  const joinAll = useMutation({ mutationFn: () => api.runScenario("join_all"), onSuccess: invalidate, onError: onErr });
  const burst = useMutation({
    mutationFn: (count: number) => api.runScenario("burst", { count, at_once: 16 }),
    onSuccess: invalidate,
    onError: onErr,
  });

  const [form, setForm] = useState({ count: 1, app_key: "", join_eui: "", class: "A", region: "EU868", default_dr: 5 });
  const [burstN, setBurstN] = useState(50);

  return (
    <div className="space-y-6">
      {err && (
        <div className="rounded-md border border-red-500/40 bg-red-500/10 px-3 py-2 text-sm text-red-400">
          {err} <button className="ml-2 underline" onClick={() => setErr(null)}>dismiss</button>
        </div>
      )}

      <Card>
        <CardTitle>Create tags</CardTitle>
        <form
          className="mt-3 grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-6"
          onSubmit={(e) => {
            e.preventDefault();
            create.mutate({
              count: Number(form.count),
              app_key: form.app_key || undefined,
              join_eui: form.join_eui || undefined,
              class: form.class,
              region: form.region,
              default_dr: Number(form.default_dr),
            });
          }}
        >
          <label className="text-xs text-muted-foreground">
            Count (fleet)
            <Input type="number" min={1} value={form.count} onChange={(e) => setForm({ ...form, count: +e.target.value })} />
          </label>
          <label className="text-xs text-muted-foreground">
            AppKey (blank = random)
            <Input placeholder="32 hex" value={form.app_key} onChange={(e) => setForm({ ...form, app_key: e.target.value })} />
          </label>
          <label className="text-xs text-muted-foreground">
            JoinEUI
            <Input placeholder="16 hex" value={form.join_eui} onChange={(e) => setForm({ ...form, join_eui: e.target.value })} />
          </label>
          <label className="text-xs text-muted-foreground">
            Class
            <Input value={form.class} onChange={(e) => setForm({ ...form, class: e.target.value })} />
          </label>
          <label className="text-xs text-muted-foreground">
            Region
            <Input value={form.region} onChange={(e) => setForm({ ...form, region: e.target.value })} />
          </label>
          <div className="flex items-end">
            <Button type="submit" variant="primary" className="w-full" disabled={create.isPending}>
              + Create
            </Button>
          </div>
        </form>
      </Card>

      <div className="flex flex-wrap items-center gap-2">
        <Button variant="primary" onClick={() => joinAll.mutate()} disabled={joinAll.isPending}>
          Join all
        </Button>
        <Button onClick={() => burst.mutate(burstN)} disabled={burst.isPending}>
          Burst
        </Button>
        <Input
          type="number"
          className="w-24"
          value={burstN}
          onChange={(e) => setBurstN(+e.target.value)}
          title="burst uplink count"
        />
        <span className="text-sm text-muted-foreground">uplinks</span>
      </div>

      <Card>
        <CardTitle>Tags ({tags.data?.length ?? 0})</CardTitle>
        <div className="mt-2 overflow-x-auto">
          <table className="w-full text-sm">
            <thead className="text-left text-xs text-muted-foreground">
              <tr className="border-b">
                <th className="py-2 pr-3">ID</th>
                <th className="py-2 pr-3">DevEUI</th>
                <th className="py-2 pr-3">Class</th>
                <th className="py-2 pr-3">Joined</th>
                <th className="py-2 pr-3">FCnt↑</th>
                <th className="py-2 pr-3">Run</th>
                <th className="py-2 pr-3">Actions</th>
              </tr>
            </thead>
            <tbody>
              {tags.data?.map((t: Tag) => (
                <tr key={t.id} className="border-b border-border/50">
                  <td className="py-2 pr-3">{t.id}</td>
                  <td className="py-2 pr-3"><Mono>{t.dev_eui}</Mono></td>
                  <td className="py-2 pr-3"><Badge>{t.class}</Badge></td>
                  <td className="py-2 pr-3">
                    {t.session?.joined ? <Badge tone="green">joined</Badge> : <Badge tone="muted">no</Badge>}
                  </td>
                  <td className="py-2 pr-3">{t.session?.fcnt_up ?? 0}</td>
                  <td className="py-2 pr-3"><Dot on={t.running} /></td>
                  <td className="flex gap-1.5 py-2 pr-3">
                    <Button onClick={() => join.mutate(t.id)} disabled={join.isPending}>Join</Button>
                    <Button onClick={() => uplink.mutate(t.id)} disabled={!t.running}>Uplink</Button>
                    <Button variant="danger" onClick={() => del.mutate(t.id)}>✕</Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Card>
    </div>
  );
}
