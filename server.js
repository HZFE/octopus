const grpc = require('grpc');
const protoLoader = require('@grpc/proto-loader');

const PROTO_PATH = __dirname + '/proto/server.proto';
const packageDefinition = protoLoader.loadSync(PROTO_PATH, {
  keepCase: true,
  longs: String,
  enums: String,
  defaults: true,
  oneofs: true,
});
const proto = grpc.loadPackageDefinition(packageDefinition).server;

/**
 * Implements the SayHello RPC method.
 */
function sayHello(call, callback) {
  console.log(JSON.stringify(call));
  callback(null, { query: call.request.query });
}

function test(call, callback) {
  console.log('test', JSON.stringify(call));
  callback(null, { test: call.request.test });
}

/**
 * Starts an RPC server that receives requests for the Greeter service at the
 * sample server port
 */
function main() {
  const server = new grpc.Server();
  server.addService(proto.Greeter.service, { sayHello, test });
  server.bind('0.0.0.0:50051', grpc.ServerCredentials.createInsecure());
  server.start();
}

main();
