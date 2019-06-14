const grpc = require('grpc');
const protoLoader = require('@grpc/proto-loader');
const PROTO_PATH = __dirname + '/service.proto';

const packageDefinition = protoLoader.loadSync(PROTO_PATH, {
  keepCase: true,
  longs: String,
  enums: String,
  defaults: true,
  oneofs: true,
});
const protoDescriptor = grpc.loadPackageDefinition(packageDefinition);
// console.log(packageDefinition, protoDescriptor);
// The protoDescriptor object has the full package hierarchy
const Greeter = protoDescriptor.helloworld.Greeter;
const Server = new grpc.Server();
const SayHello = (call) => {
  call.on('data', function(point) {
    console.log(point)
    call.write('dddd')
  });
  call.on('end', function() {
    call.end()
  });
};
Server.addProtoService(Greeter.service, {
  SayHello,
});
Server.bind('0.0.0.0:50051', grpc.ServerCredentials.createInsecure());
Server.start();
