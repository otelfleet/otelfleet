import { createFileRoute } from '@tanstack/react-router'
import { AgentDetailPage } from '../pages/AgentDetailPage'

export const Route = createFileRoute('/agents/$agentId')({
  component: RouteComponent,
})

function RouteComponent() {
  const { agentId } = Route.useParams()
  return <AgentDetailPage agentId={agentId} />
}
