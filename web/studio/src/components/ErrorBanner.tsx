/** Inline error display for API failures (wrong password, 403, etc.). */
export function ErrorBanner({ message }: { message: string | null }) {
  if (!message) return null;
  return (
    <div role="alert" data-testid="error" className="error-banner">
      {message}
    </div>
  );
}
