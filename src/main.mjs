import { createQueryServer } from './server.mjs';

const port = Number(process.env.PORT ?? 3000);

createQueryServer().listen(port, () => {
  console.log(`parambind listening on http://127.0.0.1:${port}`);
});
