import base from './vite.config'

export default {
  ...base,
  server: { host: '127.0.0.1', port: 5174, strictPort: true, proxy: { '/api': { target: 'http://127.0.0.1:8081', changeOrigin: false } } },
}
