export const LEVELS = ['A1', 'A2', 'B1', 'B2', 'C1'] as const
export type Level = (typeof LEVELS)[number]

/** Пока прогноз собирается, даты нет: честнее показать это, чем выдумать срок. */
export type Forecast = 'collecting'
