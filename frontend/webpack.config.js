// Webpack 5 — bundling de React + TypeScript, CSS, HTML y Service Worker (PWA)
const path = require('path');
const HtmlWebpackPlugin = require('html-webpack-plugin');
const MiniCssExtractPlugin = require('mini-css-extract-plugin');
const CopyWebpackPlugin = require('copy-webpack-plugin');
const { GenerateSW } = require('workbox-webpack-plugin');

module.exports = (_env, argv) => {
  const prod = argv.mode === 'production';
  return {
    entry: './src/index.tsx',
    output: {
      path: path.resolve(__dirname, 'dist'),
      filename: prod ? 'js/[name].[contenthash:8].js' : 'js/[name].js',
      publicPath: '/',
      clean: true,
    },
    resolve: { extensions: ['.tsx', '.ts', '.js'] },
    devtool: prod ? false : 'eval-source-map',
    module: {
      rules: [
        { test: /\.tsx?$/, use: 'ts-loader', exclude: /node_modules/ },
        { test: /\.css$/, use: [prod ? MiniCssExtractPlugin.loader : 'style-loader', 'css-loader'] },
        { test: /\.(png|svg|jpg|webp)$/, type: 'asset/resource' },
      ],
    },
    // Code splitting: React y dependencias en un chunk "vendors" cacheable
    optimization: {
      splitChunks: { chunks: 'all', cacheGroups: { vendors: { test: /node_modules/, name: 'vendors' } } },
      runtimeChunk: 'single',
    },
    plugins: [
      new HtmlWebpackPlugin({ template: './public/index.html', favicon: './public/icons/favicon.png' }),
      new MiniCssExtractPlugin({ filename: 'css/[name].[contenthash:8].css' }),
      new CopyWebpackPlugin({ patterns: [{ from: 'public/manifest.webmanifest' }, { from: 'public/icons', to: 'icons' }] }),
      // PWA: Service Worker generado con Workbox (solo en producción)
      ...(prod
        ? [new GenerateSW({
            clientsClaim: true,
            skipWaiting: true,
            navigateFallback: '/index.html',
            navigateFallbackDenylist: [/^\/api\//],
            runtimeCaching: [
              { urlPattern: /\/api\/catalog\//, handler: 'StaleWhileRevalidate', options: { cacheName: 'catalog-api' } },
            ],
          })]
        : []),
    ],
    devServer: {
      port: 3000,
      historyApiFallback: true,
      hot: true,
      proxy: [{ context: ['/api'], target: 'http://localhost:8080' }],
    },
    performance: { hints: false },
  };
};
