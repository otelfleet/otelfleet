import {
  EnterIcon,
  GearIcon,
  ExitIcon,
  Link2Icon,
  Component1Icon,
  ShuffleIcon,
  LayersIcon,
} from "@radix-ui/react-icons";

type IconComponent = typeof GearIcon;
import { create, type Message } from "@bufbuild/protobuf";
import type { GenMessage } from "@bufbuild/protobuf/codegenv2";
import { anyPack, anyUnpack, type Any } from "@bufbuild/protobuf/wkt";
import {
  ReceiverSchema,
  ProcessorSchema,
  ExporterSchema,
  ConnectorSchema,
  ExtensionSchema,
  PipelineSchema,
  CollectorConfigSchema,
  ComponentDefinitionSchema,
  type ComponentDefinition,
} from "../gen/api/pkg/api/resources/v1alpha1/resources_pb";

const TYPE_URL_PREFIX = "type.googleapis.com/";
const YAML_CONTENT_TYPE = "application/yaml";

const encoder = new TextEncoder();
const decoder = new TextDecoder();

export interface EntityType {
  slug: string;
  label: string;
  description: string;
  typeUrl: string;
  icon: IconComponent;
  // Individual collector components (receiver, processor, ...) as opposed to a
  // complete collector config.
  component: boolean;
  // Collector configs get the pipeline graph visualization alongside the editor.
  visualize: boolean;
  pack: (raw: string) => Any;
  unpack: (obj?: Any) => string;
}

type ComponentEntity = Message & { value?: ComponentDefinition | undefined };

// Component entities (receiver, processor, ...) all wrap a raw ComponentDefinition.
function componentEntity<T extends ComponentEntity>(
  slug: string,
  label: string,
  description: string,
  icon: IconComponent,
  schema: GenMessage<T>,
): EntityType {
  return {
    slug,
    label,
    description,
    icon,
    typeUrl: TYPE_URL_PREFIX + schema.typeName,
    component: true,
    visualize: false,
    pack: (raw) => {
      const definition = create(ComponentDefinitionSchema, {
        contentType: YAML_CONTENT_TYPE,
        value: { case: "raw", value: encoder.encode(raw) },
      });
      return anyPack(schema, create(schema, { value: definition } as Parameters<typeof create<GenMessage<T>>>[1]));
    },
    unpack: (obj) => {
      if (!obj) return "";
      const msg = anyUnpack(obj, schema);
      const value = msg?.value?.value;
      if (value?.case === "raw") return decoder.decode(value.value as Uint8Array);
      return "";
    },
  };
}

const collectorConfig: EntityType = {
  slug: "collectorconfig",
  label: "Collector",
  description: "Collector configurations",
  icon: LayersIcon,
  typeUrl: TYPE_URL_PREFIX + CollectorConfigSchema.typeName,
  component: false,
  visualize: true,
  pack: (raw) =>
    anyPack(
      CollectorConfigSchema,
      create(CollectorConfigSchema, {
        contentType: YAML_CONTENT_TYPE,
        value: { case: "Raw", value: encoder.encode(raw) },
      }),
    ),
  unpack: (obj) => {
    if (!obj) return "";
    const msg = anyUnpack(obj, CollectorConfigSchema);
    if (msg?.value?.case === "Raw") return decoder.decode(msg.value.value);
    return "";
  },
};

export const ENTITY_TYPES: EntityType[] = [
  componentEntity("receiver", "Receiver", "OpenTelemetry receivers", EnterIcon, ReceiverSchema),
  componentEntity("processor", "Processor", "OpenTelemetry processors", GearIcon, ProcessorSchema),
  componentEntity("exporter", "Exporter", "OpenTelemetry exporters", ExitIcon, ExporterSchema),
  componentEntity("connector", "Connector", "OpenTelemetry connectors", Link2Icon, ConnectorSchema),
  componentEntity("extension", "Extensions", "OpenTelemetry extensions", Component1Icon, ExtensionSchema),
  componentEntity("pipeline", "Pipelines", "OpenTelemetry pipelines", ShuffleIcon, PipelineSchema),
  collectorConfig,
];

export const COMPONENT_ENTITY_TYPES: EntityType[] = ENTITY_TYPES.filter((e) => e.component);
export const COLLECTOR_ENTITY_TYPES: EntityType[] = ENTITY_TYPES.filter((e) => !e.component);

export function getEntityType(slug: string): EntityType | undefined {
  return ENTITY_TYPES.find((e) => e.slug === slug);
}
