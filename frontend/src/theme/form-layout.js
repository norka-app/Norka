/** Shared Naive UI form rhythm. Pass these to n-grid / n-space; CSS mirrors them. */
export const FIELD_GAP = 16
export const SECTION_GAP = 24
export const BUTTON_GAP = 8

/** n-grid cols (responsive="self", numeric widths). 12 columns from 640px. */
export const FORM_COLS = '1 640:12'

export const SPAN_FULL = '1 640:12'
export const SPAN_HALF = '1 640:6'
export const SPAN_THIRD = '1 640:4'
export const SPAN_TWO_THIRDS = '1 640:8'
export const SPAN_FIVE = '1 640:5'
export const SPAN_SEVEN = '1 640:7'

export const plainInputProps = {
  autocapitalize: 'off',
  autocorrect: 'off',
  spellcheck: 'false',
}

export const requiredInputProps = {
  ...plainInputProps,
  required: true,
}
