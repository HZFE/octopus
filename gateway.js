const grpc = require('grpc');
const protoLoader = require('@grpc/proto-loader');
const Koa = require('koa');
const app = new Koa();
const koaBody = require('koa-body');
const yaml = require('js-yaml');
const fs = require('fs');

const config = yaml.safeLoad(fs.readFileSync(__dirname + '/gateway.yml', 'utf8'));

app.use(koaBody());

app.use(async ctx => {
  const { method, url, header, query, body } = ctx.request;
  let result = {
    code: 1,
    message: 'error',
  };

  for (let i = 0; i < config.length; i++) {
    const item = config[i];
    const [package, service, fun] = item.service.split('.');

    if (
      item.url !== url.split('?')[0] ||
      item.method.toLowerCase() !== method.toLowerCase()
    ) {
      continue;
    }

    const packageDefinition = protoLoader.loadSync(`${__dirname}/proto/${package}.proto`, {
      keepCase: true,
      longs: String,
      enums: String,
      defaults: true,
      oneofs: true,
    });
    const proto = grpc.loadPackageDefinition(packageDefinition)[package];
    const client = new proto[service](
      `${item.rpc.ip}:${item.rpc.port}`,
      grpc.credentials.createInsecure()
    );

    result = await new Promise((r, j) => {
      client[fun]({ ...query, ...body }, (err, response) => {
        r(JSON.stringify(response) || 'empty');
      });
    });

    break;
  }

  ctx.body = result;
});

app.listen(3000);
