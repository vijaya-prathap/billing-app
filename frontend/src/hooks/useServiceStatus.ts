import { useCallback, useEffect, useState } from "react";
import { api } from "../services/api";
import type { HealthResponse, ServiceStatus } from "../types";

const checks: { name: string; endpoint: string; run: () => Promise<HealthResponse> }[] = [
  { name: "API server", endpoint: "/health", run: api.health },
  { name: "Database", endpoint: "/ready", run: api.ready },
];

const initialStatuses = (): ServiceStatus[] =>
  checks.map(({ name, endpoint }) => ({ name, endpoint, state: "checking", detail: "", checkedAt: null }));

export function useServiceStatus(pollMs = 15000) {
  const [statuses, setStatuses] = useState<ServiceStatus[]>(initialStatuses);

  const refresh = useCallback(async () => {
    const results = await Promise.all(
      checks.map(async ({ name, endpoint, run }): Promise<ServiceStatus> => {
        try {
          const res = await run();
          return { name, endpoint, state: "up", detail: res.status, checkedAt: new Date() };
        } catch (err) {
          const detail = err instanceof Error ? err.message : "unreachable";
          return { name, endpoint, state: "down", detail, checkedAt: new Date() };
        }
      }),
    );
    setStatuses(results);
  }, []);

  useEffect(() => {
    void refresh();
    const id = window.setInterval(() => void refresh(), pollMs);
    return () => window.clearInterval(id);
  }, [refresh, pollMs]);

  return { statuses, refresh };
}
