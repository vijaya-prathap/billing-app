import { useState } from "react";
import { useAuth } from "../hooks/useAuth";
import type { AuthUser } from "../types";

function initials(user: AuthUser): string {
  const parts = user.name.trim().split(/\s+/).filter(Boolean);
  const letters = parts.length >= 2 ? parts[0][0] + parts[parts.length - 1][0] : (parts[0] ?? user.email)[0];
  return letters.toUpperCase();
}

function Avatar({ user }: { user: AuthUser }) {
  const [broken, setBroken] = useState(false);
  if (user.picture && !broken) {
    // Google profile images reject requests that carry a referrer.
    return (
      <img className="avatar" src={user.picture} alt="" referrerPolicy="no-referrer" onError={() => setBroken(true)} />
    );
  }
  return (
    <span className="avatar avatar-initials" aria-hidden="true">
      {initials(user)}
    </span>
  );
}

export function UserMenu() {
  const { user, signOut } = useAuth();
  const [busy, setBusy] = useState(false);
  if (!user) return null;

  const onSignOut = async () => {
    setBusy(true);
    try {
      await signOut();
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="user-menu">
      <Avatar user={user} />
      <div className="user-menu-text">
        <strong>{user.name}</strong>
        <span className="muted small">{user.email}</span>
      </div>
      <button type="button" onClick={() => void onSignOut()} disabled={busy}>
        {busy ? "Signing out…" : "Sign out"}
      </button>
    </div>
  );
}
