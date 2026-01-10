import { createRootRoute, Link, Outlet } from '@tanstack/react-router'
import { TanStackRouterDevtools } from '@tanstack/react-router-devtools'
import Base from '../components/Base'

const RootLayout = () => (
  <>
  <Base></Base>
    <TanStackRouterDevtools />
  </>
)

export const Route = createRootRoute({ component: RootLayout })