export function errorMessage(err: unknown): string {
  return (err as Error).message;
}
