export default { // eslint-disable-line import/no-unused-modules
  process: (content) => {
    return {code: `module.exports = ${JSON.stringify(content)}`};
  },
};
