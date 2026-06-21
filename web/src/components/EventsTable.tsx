import type { Event } from "@/lib/api";
import { Badge, Mono } from "./ui";

function kindTone(e: Event) {
  if (e.kind === "join") return e.result === "error" ? "red" : "violet";
  return e.direction === "up" ? "blue" : "green";
}

export default function EventsTable({ events }: { events: Event[] }) {
  if (!events.length) {
    return <p className="py-8 text-center text-sm text-muted-foreground">No traffic yet — join a tag and send an uplink.</p>;
  }
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead className="text-left text-xs text-muted-foreground">
          <tr className="border-b">
            <th className="py-2 pr-3 font-medium">Time</th>
            <th className="py-2 pr-3 font-medium">Tag</th>
            <th className="py-2 pr-3 font-medium">Dir</th>
            <th className="py-2 pr-3 font-medium">Kind</th>
            <th className="py-2 pr-3 font-medium">FCnt</th>
            <th className="py-2 pr-3 font-medium">FPort</th>
            <th className="py-2 pr-3 font-medium">Payload</th>
          </tr>
        </thead>
        <tbody>
          {events.map((e) => (
            <tr key={e.id || `${e.ts}-${e.kind}-${Math.random()}`} className="border-b border-border/50">
              <td className="py-1.5 pr-3 text-muted-foreground">
                <Mono>{e.ts ? new Date(e.ts).toLocaleTimeString() : "—"}</Mono>
              </td>
              <td className="py-1.5 pr-3">{e.tag_id ?? "—"}</td>
              <td className="py-1.5 pr-3">
                <Badge tone={e.direction === "up" ? "blue" : "green"}>{e.direction === "up" ? "↑ up" : "↓ down"}</Badge>
              </td>
              <td className="py-1.5 pr-3">
                <Badge tone={kindTone(e)}>{e.kind}</Badge>
              </td>
              <td className="py-1.5 pr-3">{e.fcnt ?? "—"}</td>
              <td className="py-1.5 pr-3">{e.fport ?? "—"}</td>
              <td className="py-1.5 pr-3">
                <Mono className="text-muted-foreground">{e.payload_hex || e.decoded || "—"}</Mono>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
