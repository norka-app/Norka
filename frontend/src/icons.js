// Один набор Ionicons 5 через NIcon. Размер внутри n-button не задаём:
// кнопка сама ставит высоту глифа под свой size.
import { h } from 'vue'
import { NIcon } from 'naive-ui'

export {
  Add,
  AlbumsOutline,
  ArrowDown,
  ArrowUp,
  Checkmark,
  CheckmarkCircle,
  ChevronBack,
  ChevronDown,
  ChevronForward,
  ChevronUp,
  Close,
  CloseCircle,
  ColorWandOutline,
  CopyOutline,
  DocumentTextOutline,
  EllipsisHorizontal,
  FlagOutline,
  FolderOutline,
  GitNetworkOutline,
  HourglassOutline,
  OpenOutline,
  OptionsOutline,
  Pause,
  PauseOutline,
  Power,
  Pulse,
  ReaderOutline,
  Refresh,
  RemoveCircleOutline,
  Search,
  ServerOutline,
  SpeedometerOutline,
  SwapHorizontal,
  SyncOutline,
  TrashOutline,
  Warning,
} from '@vicons/ionicons5'

export function renderIcon(icon, size) {
  const props = { 'aria-hidden': 'true' }
  if (size != null) props.size = size
  return () => h(NIcon, props, { default: () => h(icon) })
}
