import { createFileRoute } from '@tanstack/react-router'
import { ListAssignmentsPage } from '../pages/ListAssignmentsPage'

export const Route = createFileRoute('/assignments')({
  component: RouteComponent,
})

function RouteComponent() {
  return <ListAssignmentsPage />
}
