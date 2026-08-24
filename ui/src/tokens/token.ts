export interface TokenLabelValue {
  key: string;
  value: string;
}

export interface TokenValues {
  ttlSeconds: number;
  labels: TokenLabelValue[];
}

export const DEFAULT_TOKEN_TTL_SECONDS = 600;

export function emptyTokenValues(): TokenValues {
  return {
    ttlSeconds: DEFAULT_TOKEN_TTL_SECONDS,
    labels: [],
  };
}

export function emptyTokenLabel(): TokenLabelValue {
  return { key: '', value: '' };
}

export function toLabelsMap(labels: TokenLabelValue[]): { [key: string]: string } {
  const map: { [key: string]: string } = {};
  for (const { key, value } of labels) {
    const trimmed = key.trim();
    if (trimmed) map[trimmed] = value.trim();
  }
  return map;
}
