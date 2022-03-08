// This sets up the URL prefix used in webpack's chunk loading.
// This file must be imported before any lazy-loading is being attempted.
import {joinPaths} from './utils.js';
const {assetUrlPrefix} = window.config;

__webpack_public_path__ = joinPaths(assetUrlPrefix, '/');
