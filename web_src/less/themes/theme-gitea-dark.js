// configuration for the remap-css module
// see https://github.com/silverwind/remap-css#readme

module.exports.ignoreSelectors = [
  /^.chroma/,
  /^.ui.inverted/,
  /^.vue-tooltip-theme/,
  /^\[data-tooltip\]/,
];

module.exports.mappings = {
  '$color: rgba(0,0,0,.95)': 'rgba(255,255,255,.9)',
  '$color: rgba(0,0,0,.9)': 'rgba(255,255,255,.9)',
  '$color: rgba(0,0,0,.87)': 'rgba(255,255,255,.87)',
  '$color: rgba(0,0,0,.85)': 'rgba(255,255,255,.85)',
  '$color: rgba(0,0,0,.8)': 'rgba(255,255,255,.8)',
  '$color: rgba(0,0,0,.7)': 'rgba(255,255,255,.7)',
  '$color: rgba(0,0,0,.6)': 'rgba(255,255,255,.6)',
  '$color: rgba(0,0,0,.5)': 'rgba(255,255,255,.5)',
  '$color: rgba(0,0,0,.4)': 'rgba(255,255,255,.4)',
  '$color: rgba(0,0,0,.3)': 'rgba(255,255,255,.3)',
  '$color: rgba(0,0,0,.2)': 'rgba(255,255,255,.2)',
  '$color: rgba(0,0,0,.15)': 'rgba(255,255,255,.15)',
  '$color: rgba(0,0,0,.05)': 'rgba(255,255,255,.05)',
  '$color: rgba(0,0,0,.04);': 'rgba(255,255,255,.04)',
  '$color: rgba(0,0,0,.03)': 'rgba(255,255,255,.06)',
  '$color: rgba(27,31,35,0.3);': 'rgba(255,255,255,.3)',
  '$color: rgba(255,255,255, 0.1)': 'rgba(0,0,0,.1)',
  '$color: hsla(0,0%,100%,.05)': 'rgba(0,0,0,.1)',
  '$color: hsla(0,0%,86.3%,.15)': 'rgba(38,38,38,.15)',
  '$color: hsla(0,0%,86.3%,.35)': 'rgba(38,38,38,.35)',
  '$color: hsla(0,0%,100%,.65)': 'rgba(0,0,0,.65)',
  '$color: hsla(0,0%,100%,.5)': 'rgba(0,0,0,.5)',
  '$color: rgba(40,40,40,.3)': 'rgba(255,255,255,.2)',

  '$color: #000000': '#eeeeee',
  '$color: #1b1c1d': '#dddddd',
  '$color: #303030': '#cccccc',
  '$color: #333333': '#cccccc',
  '$color: #404040': '#a0a0a0',
  '$color: #444444': '#aaaaaa',
  '$color: #464646': '#bbbbbb',
  '$color: #575a68': '#909090',
  '$color: #666666': '#808080',
  '$color: #767676': '#757575',
  '$color: #838383': '#707070',
  '$color: #888888': '#707070',
  '$color: #95a5a6': '#666666',
  '$color: #999999': '#666666',
  '$color: #a6a6a6': '#555555',
  '$color: #aaaaaa': '#606060',
  '$color: #b4b4b4': '#505050',
  '$color: #bababc': '#505050',
  '$color: #bbbbbb': '#505050',
  '$color: #c0c1c2': '#4c4c4c',
  '$color: #cacbcd': '#484848',
  '$color: #cccccc': '#484848',
  '$color: #d3cfcf': '#424242',
  '$color: #d3d3d4': '#484848',
  '$color: #d4d4d5': '#424242',
  '$color: #d6d6d6': '#424242',
  '$color: #daecfe': '#3c3c3c',
  '$color: #dcddde': '#3c3c3c',
  '$color: #dddddd': '#3c3c3c',
  '$color: #e0e0e0': '#3c3c3c',
  '$color: #e0e1e2': '#3c3c3c',
  '$color: #e8e8e8': '#3a3a3a',
  '$color: #eaeaea': '#383838',
  '$color: #ebebeb': '#383838',
  '$color: #eeeeee': '#343434',
  '$color: #f0f0f0': '#2a2a2a',
  '$color: #f0f9ff': '#2a2a2a',
  '$color: #f3f3f3': '#2a2a2a',
  '$color: #f3f4f5': '#282828',
  '$color: #f5f5f5': '#262626',
  '$color: #f6f8fa': '#262626',
  '$color: #f7f7f7': '#242424',
  '$color: #f8f8f9': '#232323',
  '$color: #f9fafb': '#222222',
  '$color: #fafafa': '#202020',
  '$color: #fcfcfc': 'var(--mono-color-10)',

  '$background: #ffffff': 'var(--mono-color-10)',
  '$border: #ffffff': 'var(--mono-color-10)',

  '$box-shadow: rgba(34,36,38,.35)': 'rgba(255,255,255,.3)',
  '$box-shadow: rgba(34,36,38,.15)': 'rgba(255,255,255,.2)',

  '$color: rgba(34,36,38,.35)': 'rgba(255,255,255,.3)',
  '$color: rgba(34,36,38,.15)': 'rgba(255,255,255,.2)',

  /* primary color */
  '$color: #42402f': 'var(--primary-color)',
  '$color: #2c3e50': 'var(--primary-color)',
  '$color: #1155cc': 'var(--primary-color)',
  '$color: #0166e6': 'var(--primary-color)',
  '$color: #0087f5': 'var(--primary-color)',
  '$color: #1678c2': 'var(--primary-color)',
  '$color: #2185d0': 'var(--primary-color)',
  '$color: #4183c4': 'var(--primary-color)',
  '$color: #85b7d9': 'var(--primary-color)',

  /* primary color hover */
  '$color: #1e70bf': 'var(--primary-color-light-1)',
  '$color: #96c8da': 'var(--primary-color-light-1)',

  /* blue */
  '$color: #e6f1f6': '#182030',
  '$color: #f1f8ff': '#182030',

  /* green */
  '$color: #2c662d': '#5a5',
  '$color: #16ab39': '#373',
  '$color: #21ba45': '#373', /* signed SHAs */
  '$color: #6cc644': '#5a5',
  '$color: #1ebc30': '#373',
  '$color: #a3c293': '#373', /* signed commit */
  '$color: #99ff99': '#252', /* diff add word */
  '$color: #acf2bd': '#252', /* diff add word */
  '$color: #bef5cb': '#060',
  '$color: #c1e9c1': '#060', /* diff add */
  '$color: #cdffd8': '#083808', /* diff line num */
  '$color: #d6fcd6': '#002800', /* diff add */
  '$color: #e5f9e7': '#030',
  '$color: #e6ffed': '#030',
  '$color: #fcfff5': '#030',

  /* red */
  '$color: #ff0000': '#d82828',
  '$color: #dd1144': '#d82828',
  '$color: #db2828': '#d82828',
  '$color: #d01919': '#911',
  '$color: #9f3a38': '#d82828',
  '$color: #d95c5c': '#d82828',
  '$color: #ff9999': '#622', /* diff remove word */
  '$color: #e0b4b4': '#d82828',
  '$color: #fdb8c0': '#622', /* diff remove word */
  '$color: #f1c0c0': '#600', /* diff remove */
  '$color: #ffe5e4': '#380000',
  '$color: #ffe0e0': '#380000', /* diff remove */
  '$color: #ffe8e6': '#300',
  '$color: #ffeef0': '#300',
  '$color: #fff6f6': '#300',

  /* yellow */
  '$color: #573a08': '#cb4',
  '$color: #b58105': '#cb4',
  '$color: #fbbd08': '#bba257',
  '$color: #c9ba9b': '#cb4',
  '$color: #fff866': '#cb4', /* code highlight */
  '$color: #fffbdd': '#383418', /* line highlight */
  '$color: #f9edbe': '#383418',
  '$color: #fff8db': '#383418',
  '$color: #fffaf3': '#282418',
  '$color: #fcf8e9': '#232002', /* private repo on frontpage */

  /* purple */
  '$color: #a333c8': '#73589a',

  /* orange */
  '$color: #f2711c': '#fb8532',
  '$color: #ffedde': '#320',

  /* other stuff */
  '$background: #ffffee': 'rgba(255,255,255,.05)', /* file row hover */
  '$background: linear-gradient(to right, rgba(255, 255, 255, 0), #ffffff 100%)': 'linear-gradient(to right, rgba(255, 255, 255, 0), #202020 100%)',
  '$background: linear-gradient(90deg, hsla(0,0%,100%,0),#fff)': 'linear-gradient(90deg, hsla(0,0%,90%,0),#202020)',
};
