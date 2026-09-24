import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import type { ReactNode } from "react";
import { api, ApiError, AUTH_EXPIRED_EVENT, AUTH_REVOKED_EVENT } from "../services/api";
import type { AuthUser } from "../types";

// "disabled" means the backend runs with AUTH_DISABLED=true; the app then skips the login screen.
export type AuthStatus = "loading" | "error" | "disabled" | "anonymous" | "authenticated";

// Why the user is looking at the login screen, so it can say so instead of silently bouncing them.
// "denied", "failed" and "cancelled" arrive as ?auth_error=… from the OAuth callback.
export type AuthNotice = "expired" | "signed_out" | "revoked" | "denied" | "failed" | "cancelled" | null;

const AUTH_ERROR_PARAM = "auth_error";
const callbackNotices: ReadonlySet<string> = new Set(["denied", "failed", "cancelled"]);

// Reads the callback's auth_error marker once and removes it from the address bar,
// so a reload or a shared link does not show a stale message.
function takeCallbackNotice(): AuthNotice {
  const params = new URLSearchParams(window.location.search);
  const value = params.get(AUTH_ERROR_PARAM);
  if (!value) return null;
  params.delete(AUTH_ERROR_PARAM);
  const query = params.toString();
  window.history.replaceState(null, "", `${window.location.pathname}${query ? `?${query}` : ""}${window.location.hash}`);
  return callbackNotices.has(value) ? (value as AuthNotice) : "failed";
}

interface AuthContextValue {
  status: AuthStatus;
  user: AuthUser | null;
  error: unknown;
  notice: AuthNotice;
  signOut: () => Promise<void>;
  retry: () => void;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [status, setStatus] = useState<AuthStatus>("loading");
  const [user, setUser] = useState<AuthUser | null>(null);
  const [error, setError] = useState<unknown>(null);
  const [notice, setNotice] = useState<AuthNotice>(() => takeCallbackNotice());
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setStatus("loading");
    setError(null);

    (async () => {
      try {
        const config = await api.auth.config();
        if (cancelled) return;
        if (!config.enabled) {
          setStatus("disabled");
          return;
        }
        try {
          const { user } = await api.auth.me();
          if (cancelled) return;
          setUser(user);
          setNotice(null);
          setStatus("authenticated");
        } catch (err) {
          if (cancelled) return;
          if (err instanceof ApiError && err.status === 401) {
            setStatus("anonymous");
          } else if (err instanceof ApiError && err.status === 403) {
            // Valid session, but the account has since been deactivated.
            setNotice("revoked");
            setStatus("anonymous");
          } else {
            throw err;
          }
        }
      } catch (err) {
        if (cancelled) return;
        setError(err);
        setStatus("error");
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [attempt]);

  useEffect(() => {
    const kick = (why: AuthNotice) => () => {
      setStatus((s) => {
        if (s !== "authenticated") return s;
        setUser(null);
        setNotice(why);
        return "anonymous";
      });
    };
    const onExpired = kick("expired");
    const onRevoked = kick("revoked");
    window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
    window.addEventListener(AUTH_REVOKED_EVENT, onRevoked);
    return () => {
      window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
      window.removeEventListener(AUTH_REVOKED_EVENT, onRevoked);
    };
  }, []);

  const signOut = useCallback(async () => {
    try {
      await api.auth.logout();
    } finally {
      setUser(null);
      setNotice("signed_out");
      setStatus("anonymous");
    }
  }, []);

  const retry = useCallback(() => setAttempt((n) => n + 1), []);

  const value = useMemo<AuthContextValue>(
    () => ({ status, user, error, notice, signOut, retry }),
    [status, user, error, notice, signOut, retry],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used inside <AuthProvider>");
  return ctx;
}
