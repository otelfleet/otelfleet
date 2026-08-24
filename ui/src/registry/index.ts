import { createRegistry, type DescFile, type Registry } from "@bufbuild/protobuf";

// Generated modules export their file descriptor as `file_<proto path>`.
const FILE_DESCRIPTOR_EXPORT = /^file_/;

/**
 * Registry of every generated proto file under src/gen.
 *
 * Needed wherever `google.protobuf.Any` is encoded or decoded as JSON: the
 * concrete message type has to be resolvable at runtime, otherwise encoding
 * fails with "<type url> is not in the type registry".
 */
export function createGeneratedRegistry(): Registry {
  const modules = import.meta.glob<Record<string, unknown>>("../gen/**/*_pb.ts", { eager: true });
  const files = Object.values(modules).flatMap((module) =>
    Object.entries(module)
      .filter(([name]) => FILE_DESCRIPTOR_EXPORT.test(name))
      .map(([, descriptor]) => descriptor as DescFile),
  );
  return createRegistry(...files);
}

export const generatedRegistry: Registry = createGeneratedRegistry();
