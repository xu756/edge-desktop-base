import { RouterProvider, createRouter } from '@tanstack/react-router'
import { WML } from '@wailsio/runtime'
import ReactDOM from 'react-dom/client'
import { routeTree } from './routeTree.gen'

const router = createRouter({
  routeTree,
  defaultPreload: 'intent',
  scrollRestoration: true,
})

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}
// Wire up data-wml-openURL links (logos + footer "Docs" link) once the DOM is ready.
WML.Enable()

const rootElement = document.getElementById('app')!

if (!rootElement.innerHTML) {
  const root = ReactDOM.createRoot(rootElement)
  root.render(<RouterProvider router={router} />)
}
