import assert from 'node:assert/strict'
import { resolvePluginBase } from './src/plugin-base.js'

assert.equal(
  resolvePluginBase({
    apiURL: 'https://spr.local/',
    pluginURI: 'spr-herdr',
    baseURI: 'about:srcdoc'
  }).toString(),
  'https://spr.local/plugins/spr-herdr/'
)

assert.equal(
  resolvePluginBase({
    apiURL: undefined,
    pluginURI: undefined,
    baseURI: 'https://spr.local/admin/custom_plugin/spr-herdr/'
  }).toString(),
  'https://spr.local/plugins/spr-herdr/'
)

assert.equal(
  resolvePluginBase({
    apiURL: 'https://remote.example/api/',
    pluginURI: 'spr-herdr',
    baseURI: 'https://spr.local/admin/custom_plugin/spr-herdr/'
  }).toString(),
  'https://remote.example/api/plugins/spr-herdr/'
)

console.log('plugin base URL checks passed')
