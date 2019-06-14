const grpc = require('grpc');
const protoLoader = require('@grpc/proto-loader');
const Koa = require('koa');
const app = new Koa();
const koaBody = require('koa-body');
const PROTO_PATH = __dirname + '/service.proto';
const yaml = require('js-yaml');
const fs = require('fs');

const config = yaml.safeLoad(fs.readFileSync(__dirname + '/gateway.yml', 'utf8'));

const packageDefinition = protoLoader.loadSync(PROTO_PATH, {
  keepCase: true,
  longs: String,
  enums: String,
  defaults: true,
  oneofs: true,
});
const proto = grpc.loadPackageDefinition(packageDefinition).helloworld;

app.use(koaBody());
app.use(async ctx => {
  const { method, url, header, query, body } = ctx.request;
  let result = {
    code: 1,
    message: 'error',
  };

  for (let i = 0; i < config.length; i++) {
    const item = config[i];

    if (item.url !== url.split('?')[0] || item.method.toLowerCase() !== method.toLowerCase()) {
      continue;
    }

    const client = new proto[item.service]('localhost:8081', grpc.credentials.createInsecure());
    result = await new Promise((r, j) => {
      client[item.function]({ query: query.query }, (err, response) => {
        r(JSON.stringify(response));
      });
    });
  }

  ctx.body = result;
});

app.listen(3000);
