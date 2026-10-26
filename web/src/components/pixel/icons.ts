import type { PixelArt } from '@/lib/pixel'

export type IconName =
  | 'today'
  | 'cards'
  | 'exercises'
  | 'words'
  | 'mistakes'
  | 'progress'
  | 'settings'
  | 'sources'
  | 'camp'
  | 'sheets'
  | 'read'

export const ICON_SIZE = 12

/** Иконки 12×12 одного цвета: «X» закрашено цветом текста (currentColor), «.» пусто. */
const ROWS: Record<IconName, string[]> = {
  today: [
    '.....XX.....',
    '.X...XX...X.',
    '..X......X..',
    '....XXXX....',
    '....XXXX....',
    'XX..XXXX..XX',
    'XX..XXXX..XX',
    '....XXXX....',
    '....XXXX....',
    '..X......X..',
    '.X...XX...X.',
    '.....XX.....',
  ],
  cards: [
    '...XXXXXXXX.',
    '...X......X.',
    'XXXXXXXX..X.',
    'X......X..X.',
    'X.XXXX.X..X.',
    'X......X.XX.',
    'X.XXXX.XXX..',
    'X......X....',
    'X.XX...X....',
    'X......X....',
    'XXXXXXXX....',
    '............',
  ],
  exercises: [
    '.........XX.',
    '........XXXX',
    '.......XXXX.',
    '......XXXX..',
    '.....XXXX...',
    '....XXXX....',
    '...XXXX.....',
    '..XXXX......',
    '.XXXX.......',
    '.XXX........',
    'XXX.........',
    'XX..........',
  ],
  words: [
    '............',
    '.XXXXX.XXXXX',
    '.X...X.X...X',
    '.X.XXX.XXX.X',
    '.X...X.X...X',
    '.X.XXX.XXX.X',
    '.X...X.X...X',
    '.X.XXX.XXX.X',
    '.X...X.X...X',
    '.XXXXXXXXXXX',
    '............',
    '............',
  ],
  mistakes: [
    '...XXXXXX...',
    '..XX....XX..',
    '.XX..XX..XX.',
    'XX...XX...XX',
    'X....XX....X',
    'X....XX....X',
    'X....XX....X',
    'XX........XX',
    '.XX..XX..XX.',
    '..XX.XX.XX..',
    '...XXXXXX...',
    '............',
  ],
  progress: [
    '............',
    '..........XX',
    '..........XX',
    '......XX..XX',
    '......XX..XX',
    '..XX..XX..XX',
    '..XX..XX..XX',
    '..XX..XX..XX',
    '..XX..XX..XX',
    'XXXXXXXXXXXX',
    '............',
    '............',
  ],
  settings: [
    '....XXXX....',
    '.X..XXXX..X.',
    '.XX.XXXX.XX.',
    '..XXXXXXXX..',
    'XXXXX..XXXXX',
    'XXXX....XXXX',
    'XXXX....XXXX',
    'XXXXX..XXXXX',
    '..XXXXXXXX..',
    '.XX.XXXX.XX.',
    '.X..XXXX..X.',
    '....XXXX....',
  ],
  camp: [
    '.....XX.....',
    '....XXXX....',
    '...XXXXXX...',
    '..XXXXXXXX..',
    '.XXXXXXXXXX.',
    'XXXXX..XXXXX',
    'XXXX....XXXX',
    'XXX......XXX',
    'XXX......XXX',
    'XX........XX',
    'X..........X',
    '............',
  ],
  sheets: [
    '.XXXXXXXXX..',
    '.X.......XX.',
    '.X.XXXXX.X.X',
    '.X.......XXX',
    '.X.XXXXX...X',
    '.X.......X.X',
    '.X.XXXXX.X.X',
    '.X.......X.X',
    '.X.XXX...X.X',
    '.X.......X.X',
    '.XXXXXXXXX.X',
    '..........XX',
  ],
  read: [
    '............',
    '.XXXXX.XXXXX',
    'XX...XXX...X',
    'X.XXX.X.XXX.',
    'X.....X.....',
    'X.XXX.X.XXX.',
    'X.....X.....',
    'X.XXX.X.XXX.',
    'X.....X.....',
    'XXXXXXXXXXXX',
    '............',
    '............',
  ],
  sources: [
    '.XXXXXXX....',
    '.X.....XX...',
    '.X.....X.X..',
    '.X.....XXXX.',
    '.X.XXXX...X.',
    '.X........X.',
    '.X.XXXXXX.X.',
    '.X........X.',
    '.X.XXXXXX.X.',
    '.X........X.',
    '.XXXXXXXXXX.',
    '............',
  ],
}

export const ICON_NAMES = Object.keys(ROWS) as IconName[]

export function iconArt(name: IconName): PixelArt {
  return { palette: { X: 'currentColor' }, rows: ROWS[name] }
}
