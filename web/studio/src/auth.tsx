/**
 * Auth context: resolves the current user from GET /api/me on mount and exposes
 * login/register/logout that update it. The HttpOnly session cookie is the source
 * of truth; we just mirror the resolved user in memory for the shell + guard.
 */
import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from "react";
import { ApiError, api, type User } from "./api";

type AuthState = {
  user: User | null;
  loading: boolean;
  login: (username: string, password: string) => Promise<void>;
  register: (input: { username: string; displayName: string; password: string; email?: string }) => Promise<void>;
  logout: () => Promise<void>;
  refresh: () => Promise<void>;
};

const AuthContext = createContext<AuthState | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    try {
      const u = await api.me();
      setUser(u);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        setUser(null);
      } else {
        // Network or unexpected error: treat as logged out for guard purposes.
        setUser(null);
      }
    }
  }, []);

  useEffect(() => {
    void (async () => {
      await refresh();
      setLoading(false);
    })();
  }, [refresh]);

  const login = useCallback(async (username: string, password: string) => {
    const u = await api.login({ username, password });
    setUser(u);
  }, []);

  const register = useCallback(
    async (input: { username: string; displayName: string; password: string; email?: string }) => {
      await api.register(input);
      // Register does not set a session; log in immediately to get the cookie.
      const u = await api.login({ username: input.username, password: input.password });
      setUser(u);
    },
    [],
  );

  const logout = useCallback(async () => {
    await api.logout();
    setUser(null);
  }, []);

  return (
    <AuthContext.Provider value={{ user, loading, login, register, logout, refresh }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}
