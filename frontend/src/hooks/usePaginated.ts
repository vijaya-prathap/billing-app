import { useCallback, useEffect, useState } from "react";
import { errorMessage } from "../services/api";
import type { Paginated } from "../types";

interface State<T> {
  rows: T[];
  page: number;
  totalPages: number;
  total: number;
  loading: boolean;
  error: string | null;
}

// Drives a paginated list page: loads on mount, re-loads when `page` or `deps` change,
// and exposes `reload` so mutations can refresh in place.
export function usePaginated<T>(
  fetcher: (page: number) => Promise<Paginated<T>>,
  deps: unknown[] = [],
) {
  const [page, setPage] = useState(1);
  const [state, setState] = useState<State<T>>({
    rows: [],
    page: 1,
    totalPages: 1,
    total: 0,
    loading: true,
    error: null,
  });

  const load = useCallback(async () => {
    setState((s) => ({ ...s, loading: true, error: null }));
    try {
      const res = await fetcher(page);
      setState({
        rows: res.data,
        page: res.page,
        totalPages: Math.max(res.total_pages, 1),
        total: res.total,
        loading: false,
        error: null,
      });
    } catch (err) {
      setState((s) => ({ ...s, loading: false, error: errorMessage(err) }));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [fetcher, page, ...deps]);

  useEffect(() => {
    void load();
  }, [load]);

  // Deleting the last row on a page should step back rather than show an empty page.
  const reloadAfterDelete = useCallback(() => {
    if (state.rows.length === 1 && page > 1) setPage(page - 1);
    else void load();
  }, [state.rows.length, page, load]);

  return { ...state, setPage, reload: load, reloadAfterDelete };
}
