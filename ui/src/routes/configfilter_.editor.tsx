import { createFileRoute } from '@tanstack/react-router'
import { ConfigFilterEditor } from '../configfilters/ConfigFilterEditor'

type EditorSearch = {
  key?: string
}

export const Route = createFileRoute('/configfilter_/editor')({
  validateSearch: (search: Record<string, unknown>): EditorSearch => ({
    key: typeof search.key === 'string' ? search.key : undefined,
  }),
  component: RouteComponent,
})

function RouteComponent() {
  const { key } = Route.useSearch()
  return <ConfigFilterEditor entityKey={key} />
}
