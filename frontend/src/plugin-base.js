export const resolvePluginBase = ({ apiURL, pluginURI, baseURI }) => {
  const documentBase = new URL(baseURI)
  const apiBase = new URL(apiURL || '/', documentBase)
  const uri = pluginURI || 'spr-herdr'
  return new URL(`plugins/${encodeURIComponent(uri)}/`, apiBase)
}
