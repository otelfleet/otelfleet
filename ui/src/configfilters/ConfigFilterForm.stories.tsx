import type { Meta, StoryObj } from '@storybook/tanstack-react';
import { ConfigFilterForm } from './ConfigFilterForm';
import { emptyConfigFilterValues } from './configFilter';
import { MatchType } from '../gen/api/pkg/api/resources/v1alpha1/resources_pb';

/**
 * Form for a single ConfigFilter resource: which collector config to hand out,
 * and the label filters an agent must match to receive it. Label rows are
 * grouped per match type, mirroring the repeated LabelFilter on the proto.
 */
const meta = {
  title: 'ConfigFilters/ConfigFilterForm',
  component: ConfigFilterForm,
  args: {
    collectorConfigs: ['linux-baseline', 'windows-baseline', 'edge-debug'],
    onSubmit: () => {},
    onCancel: () => {},
  },
  decorators: [
    (Story) => (
      <div style={{ padding: 16, maxWidth: 900 }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof ConfigFilterForm>;

export default meta;

type Story = StoryObj<typeof meta>;

/**
 * Creating a new assignment starts with one empty label filter.
 */
export const Create: Story = {
  args: {
    initialValues: emptyConfigFilterValues(),
    editing: false,
  },
};

/**
 * Editing an existing assignment. The name is locked and the label filters are
 * populated from the stored resource.
 */
export const Edit: Story = {
  args: {
    editing: true,
    initialValues: {
      name: 'production-linux',
      isDefault: false,
      requiresApproval: true,
      configRef: 'linux-baseline',
      filters: [
        {
          type: MatchType.EQ,
          labels: [
            { scope: 'identifying', key: 'os.type', value: 'linux' },
            { scope: 'otelfleet', key: 'environment', value: 'production' },
          ],
        },
        {
          type: MatchType.RE,
          labels: [{ scope: 'nonIdentifying', key: 'host.name', value: '^edge-.*' }],
        },
      ],
    },
  },
};

/**
 * The default assignment applies to every agent no other filter matches.
 */
export const DefaultAssignment: Story = {
  args: {
    editing: true,
    initialValues: {
      ...emptyConfigFilterValues(),
      name: 'fleet-default',
      isDefault: true,
      configRef: 'linux-baseline',
    },
  },
};
