import { useEffect, useRef, useState } from "react";
import type { Event } from "./api";

interface WSMessage {
  type: string;
  event?: Event;
}

// useEventFeed subscribes to /ws and keeps the most recent events in state,
// reconnecting automatically. seed primes the buffer from the REST history.
export function useEventFeed(seed: Event[], max = 200) {
  const [events, setEvents] = useState<Event[]>(seed);
  const seededRef = useRef(false);

  useEffect(() => {
    if (!seededRef.current && seed.length) {
      setEvents(seed);
      seededRef.current = true;
    }
  }, [seed]);

  useEffect(() => {
    let ws: WebSocket | null = null;
    let closed = false;
    let retry: ReturnType<typeof setTimeout>;

    const connect = () => {
      const proto = location.protocol === "https:" ? "wss" : "ws";
      ws = new WebSocket(`${proto}://${location.host}/ws`);
      ws.onmessage = (e) => {
        try {
          const msg: WSMessage = JSON.parse(e.data);
          if (msg.type === "event" && msg.event) {
            setEvents((prev) => [msg.event!, ...prev].slice(0, max));
          }
        } catch {
          /* ignore malformed */
        }
      };
      ws.onclose = () => {
        if (!closed) retry = setTimeout(connect, 2000);
      };
    };
    connect();

    return () => {
      closed = true;
      clearTimeout(retry);
      ws?.close();
    };
  }, [max]);

  return events;
}
