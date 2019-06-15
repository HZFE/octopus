import * as grpc from 'grpc';
import * as protoLoader from '@grpc/proto-loader';
import * as Koa from 'koa';
import * as koaBody from 'koa-body';
import * as yaml from 'js-yaml';
import * as fs from 'fs';

const config = yaml.safeLoad(fs.readFileSync(__dirname + '/gateway.yml', 'utf8'));

const app = new Koa();

app.use(koaBody());

app.use(async ctx => {
  const { method, url, header, query, body } = ctx.request;
  let result = { code: 1, message: 'ERROR' };

  for (let i = 0; i < config.length; i++) {
    const item = config[i];
    const [packageName, serviceName, functionName] = item.service.split('.');

    if (
      item.url !== url.split('?')[0] ||
      item.method.toLowerCase() !== method.toLowerCase()
    ) {
      continue;
    }

    const packageDefinition = protoLoader.loadSync(`${__dirname}/proto/${packageName}.proto`, {
      keepCase: true,
      longs: String,
      enums: String,
      defaults: true,
      oneofs: true,
    });
    const proto = grpc.loadPackageDefinition(packageDefinition)[packageName];
    const client = new proto[serviceName](
      `${item.rpc.ip}:${item.rpc.port}`,
      grpc.credentials.createInsecure(),
    );

    result = await new Promise((res, rej) => {
      client[functionName]({ ...query, ...body }, (err: any, response: any) => {
        if (err) {
          return rej({ code: 2, message: JSON.stringify(err) });
        }
        res(response || { code: 0, message: 'empty' });
      });
    });

    break;
  }

  ctx.body = result;
});

app.listen(3000);
