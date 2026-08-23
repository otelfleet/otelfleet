import { createFileRoute } from '@tanstack/react-router'
import { ConfigAssignmentPage } from '../configassignment/ConfigAssignmentPage'

export const Route = createFileRoute('/configfilter')({
  component: RouteComponent,
})

function RouteComponent() {
  return <ConfigAssignmentPage />
}
