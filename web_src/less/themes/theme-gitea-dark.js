// configuration for the remap-css module
// see https://github.com/silverwind/remap-css#readme

module.exports.ignoreSelectors = [
  /^.chroma/,
  /^.ui.inverted/,
  /^.vue-tooltip-theme/,
  /^\[data-tooltip\]/,
];

module.exports.mappings = {
  '$color: rgba(0,0,0,.95)': 'var(--mono-color-100-alpha-85)',
  '$color: rgba(0,0,0,.9)': 'var(--mono-color-100-alpha-80)',
  '$color: rgba(0,0,0,.87)': 'var(--mono-color-100-alpha-75)',
  '$color: rgba(0,0,0,.85)': 'var(--mono-color-100-alpha-75)',
  '$color: rgba(0,0,0,.8)': 'var(--mono-color-100-alpha-70)',
  '$color: rgba(0,0,0,.7)': 'var(--mono-color-100-alpha-60)',
  '$color: rgba(0,0,0,.6)': 'var(--mono-color-100-alpha-50)',
  '$color: rgba(0,0,0,.5)': 'var(--mono-color-100-alpha-40)',
  '$color: rgba(0,0,0,.4)': 'var(--mono-color-100-alpha-30)',
  '$color: rgba(0,0,0,.3)': 'var(--mono-color-100-alpha-20)',
  '$color: rgba(0,0,0,.2)': 'var(--mono-color-100-alpha-15)',
  '$color: rgba(0,0,0,.15)': 'var(--mono-color-100-alpha-10)',
  '$color: rgba(0,0,0,.05)': 'var(--mono-color-100-alpha-5)',
  '$color: rgba(0,0,0,.04);': 'var(--mono-color-100-alpha-5)',
  '$color: rgba(0,0,0,.03)': 'var(--mono-color-100-alpha-5)',
  '$color: rgba(27,31,35,0.3);': 'var(--mono-color-100-alpha-30)',
  '$color: rgba(255,255,255, 0.1)': 'var(--mono-color-0-alpha-10)',
  '$color: hsla(0,0%,100%,.05)': 'var(--mono-color-0-alpha-10)',
  '$color: hsla(0,0%,86.3%,.15)': 'var(--mono-color-0-alpha-10)',
  '$color: hsla(0,0%,86.3%,.35)': 'var(--mono-color-0-alpha-30)',
  '$color: hsla(0,0%,100%,.65)': 'var(--mono-color-0-alpha-65)',
  '$color: hsla(0,0%,100%,.5)': 'var(--mono-color-0-alpha-50)',
  '$color: rgba(40,40,40,.3)': 'var(--mono-color-100-alpha-20)',

  '$color: #000000': 'var(--mono-color-88)',
  '$color: #1b1c1d': 'var(--mono-color-80)',
  '$color: #303030': 'var(--mono-color-72)',
  '$color: #333333': 'var(--mono-color-72)',
  '$color: #404040': 'var(--mono-color-66)',
  '$color: #444444': 'var(--mono-color-66)',
  '$color: #464646': 'var(--mono-color-66)',
  '$color: #575a68': 'var(--mono-color-58)',
  '$color: #666666': 'var(--mono-color-50)',
  '$color: #767676': 'var(--mono-color-46)',
  '$color: #838383': 'var(--mono-color-46)',
  '$color: #888888': 'var(--mono-color-46)',
  '$color: #95a5a6': 'var(--mono-color-42)',
  '$color: #999999': 'var(--mono-color-42)',
  '$color: #a6a6a6': 'var(--mono-color-40)',
  '$color: #aaaaaa': 'var(--mono-color-40)',
  '$color: #b4b4b4': 'var(--mono-color-36)',
  '$color: #bababc': 'var(--mono-color-36)',
  '$color: #bbbbbb': 'var(--mono-color-36)',
  '$color: #c0c1c2': 'var(--mono-color-36)',
  '$color: #cacbcd': 'var(--mono-color-32)',
  '$color: #cccccc': 'var(--mono-color-32)',
  '$color: #d3cfcf': 'var(--mono-color-32)',
  '$color: #d3d3d4': 'var(--mono-color-32)',
  '$color: #d4d4d5': 'var(--mono-color-28)',
  '$color: #d6d6d6': 'var(--mono-color-28)',
  '$color: #daecfe': 'var(--mono-color-25)',
  '$color: #dcddde': 'var(--mono-color-25)',
  '$color: #dddddd': 'var(--mono-color-25)',
  '$color: #e0e0e0': 'var(--mono-color-25)',
  '$color: #e0e1e2': 'var(--mono-color-25)',
  '$color: #e8e8e8': 'var(--mono-color-25)',
  '$color: #eaeaea': 'var(--mono-color-23)',
  '$color: #eaecef': 'var(--mono-color-23)',
  '$color: #ebebeb': 'var(--mono-color-23)',
  '$color: #eeeeee': 'var(--mono-color-20)',
  '$color: #f0f0f0': 'var(--mono-color-17)',
  '$color: #f0f9ff': 'var(--mono-color-17)',
  '$color: #f3f3f3': 'var(--mono-color-17)',
  '$color: #f3f4f5': 'var(--mono-color-16)',
  '$color: #f5f5f5': 'var(--mono-color-15)',
  '$color: #f6f8fa': 'var(--mono-color-15)',
  '$color: #f7f7f7': 'var(--mono-color-14)',
  '$color: #f8f8f9': 'var(--mono-color-14)',
  '$color: #f9fafb': 'var(--mono-color-13)',
  '$color: #fafafa': 'var(--mono-color-12)',
  '$color: #fcfcfc': 'var(--mono-color-10)',

  '$background: #ffffff': 'var(--mono-color-10)',
  '$border: #ffffff': 'var(--mono-color-10)',

  '$box-shadow: rgba(34,36,38,.35)': 'var(--mono-color-100-alpha-30)',
  '$box-shadow: rgba(34,36,38,.15)': 'var(--mono-color-100-alpha-20)',

  '$color: rgba(34,36,38,.35)': 'var(--mono-color-100-alpha-30)',
  '$color: rgba(34,36,38,.15)': 'var(--mono-color-100-alpha-20)',

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
  '$color: #e6f1f6': 'var(--blue-color-10)',
  '$color: #f1f8ff': 'var(--blue-color-10)',

  /* green */
  '$color: #2c662d': 'var(--green-color-50)',
  '$color: #16ab39': 'var(--green-color-30)',
  '$color: #21ba45': 'var(--green-color-30)', /* signed SHAs */
  '$color: #6cc644': 'var(--green-color-50)',
  '$color: #1ebc30': 'var(--green-color-30)',
  '$color: #a3c293': 'var(--green-color-30)', /* signed commit */
  '$color: #99ff99': 'var(--green-color-20)', /* diff add word */
  '$color: #acf2bd': 'var(--green-color-20)', /* diff add word */
  '$color: #bef5cb': 'var(--green-color-15)',
  '$color: #c1e9c1': 'var(--green-color-15)', /* diff add */
  '$color: #cdffd8': 'var(--green-color-12)', /* diff line num */
  '$color: #d6fcd6': 'var(--green-color-8)', /* diff add */
  '$color: #e5f9e7': 'var(--green-color-10)',
  '$color: #e6ffed': 'var(--green-color-10)',
  '$color: #fcfff5': 'var(--green-color-10)',

  /* red */
  '$color: #ff0000': 'var(--red-color-50)',
  '$color: #dd1144': 'var(--red-color-50)',
  '$color: #db2828': 'var(--red-color-50)',
  '$color: #d01919': 'var(--red-color-30)',
  '$color: #9f3a38': 'var(--red-color-50)',
  '$color: #d95c5c': 'var(--red-color-50)',
  '$color: #ff9999': 'var(--red-color-20)', /* diff remove word */
  '$color: #e0b4b4': 'var(--red-color-50)',
  '$color: #fdb8c0': 'var(--red-color-20)', /* diff remove word */
  '$color: #f1c0c0': 'var(--red-color-15)', /* diff remove */
  '$color: #ffe5e4': 'var(--red-color-10)',
  '$color: #ffe0e0': 'var(--red-color-10)', /* diff remove */
  '$color: #ffe8e6': 'var(--red-color-8)',
  '$color: #ffeef0': 'var(--red-color-8)',
  '$color: #fff6f6': 'var(--red-color-8)',

  /* yellow */
  '$color: #573a08': 'var(--yellow-color-50)',
  '$color: #b58105': 'var(--yellow-color-50)',
  '$color: #fbbd08': 'var(--yellow-color-30)',
  '$color: #c9ba9b': 'var(--yellow-color-50)',
  '$color: #fff866': 'var(--yellow-color-50)', /* code highlight */
  '$color: #fffbdd': 'var(--yellow-color-15)', /* line highlight */
  '$color: #f9edbe': 'var(--yellow-color-15)',
  '$color: #fff8db': 'var(--yellow-color-15)',
  '$color: #fffaf3': 'var(--yellow-color-10)',
  '$color: #fcf8e9': 'var(--yellow-color-8)', /* private repo on frontpage */

  /* purple */
  '$color: #a333c8': 'var(--purple-color-50)',

  /* orange */
  '$color: #f2711c': 'var(--orange-color-50)',
  '$color: #ffedde': 'var(--orange-color-10)',

  /* other stuff */
  '$background: #ffffee': 'var(--mono-color-100-alpha-5)', /* file row hover */
  '$background: linear-gradient(to right, rgba(255, 255, 255, 0), #ffffff 100%)': 'linear-gradient(to right, var(--mono-color-100-alpha-0), var(--mono-color-12) 100%)',
  '$background: linear-gradient(90deg, hsla(0,0%,100%,0),#fff)': 'linear-gradient(90deg, var(--mono-color-100-alpha-0), var(--mono-color-12))',
};
