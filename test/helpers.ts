// Integration-test helpers: spin up a throwaway JetStream-enabled NATS broker
// via Testcontainers (Docker), and reset the server's module singletons.
import {
  GenericContainer,
  Wait,
  type StartedTestContainer,
} from "testcontainers";
import { closeNats } from "../src/nats-client.js";
import { resetStreamManagerForTests } from "../src/stream-manager.js";
import { resetIdentityForTests } from "../src/identity.js";

export interface NatsHandle {
  url: string;
  container: StartedTestContainer;
}

/** Start a fresh NATS server with JetStream enabled; resolves once it's ready. */
export async function startNats(): Promise<NatsHandle> {
  const container = await new GenericContainer("nats:2.10-alpine")
    .withCommand(["-js"]) // enable JetStream
    .withExposedPorts(4222)
    .withWaitStrategy(Wait.forLogMessage(/Server is ready/))
    .start();

  const url = `nats://${container.getHost()}:${container.getMappedPort(4222)}`;
  return { url, container };
}

/** Tear down the shared connection and clear all in-process singleton state. */
export async function resetClientState(): Promise<void> {
  await closeNats();
  resetStreamManagerForTests();
  resetIdentityForTests();
}
