import { createFileRoute, notFound } from '@tanstack/react-router'
import { ResourceEditor } from '../resources/ResourceEditor'
import { getEntityType } from '../resources/entityTypes'

type EditorSearch = {
  key?: string
}

export const Route = createFileRoute('/resources/$type_/editor')({
  validateSearch: (search: Record<string, unknown>): EditorSearch => ({
    key: typeof search.key === 'string' ? search.key : undefined,
  }),
  component: RouteComponent,
})

function RouteComponent() {
  const { type } = Route.useParams()
  const { key } = Route.useSearch()
  const entityType = getEntityType(type)
  if (!entityType) throw notFound()
  return <ResourceEditor entityType={entityType} entityKey={key} />
}
