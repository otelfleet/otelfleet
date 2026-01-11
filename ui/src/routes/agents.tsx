import { createFileRoute, Outlet, useMatch } from '@tanstack/react-router'
import { AgentPage } from '../pages/ListAgentPage'

export const Route = createFileRoute('/agents')({
  component: RouteComponent,
})

function RouteComponent() {
  // Check if we're on an exact /agents route or a child route
  const match = useMatch({ from: '/agents/$agentId', shouldThrow: false })

  // If we have a child route match, render the Outlet (child route)
  if (match) {
    return <Outlet />
  }

  // Otherwise render the agents list
  return <AgentPage />
}
