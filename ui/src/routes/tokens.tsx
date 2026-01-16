import { createFileRoute, Outlet, useMatch } from '@tanstack/react-router'
import { TokenPage } from '../pages/ListTokenPage'

export const Route = createFileRoute('/tokens')({
  component: RouteComponent,
})

function RouteComponent() {
  // Check if we're on an exact /tokens route or a child route
  const match = useMatch({ from: '/tokens/$tokenId', shouldThrow: false })

  // If we have a child route match, render the Outlet (child route)
  if (match) {
    return <Outlet />
  }

  // Otherwise render the tokens list
  return <TokenPage />
}
