import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  vus: 5,
  duration: '20s',
};

const baseURL = __ENV.BASE_URL || 'http://127.0.0.1:8080';
const apiKey = __ENV.GATEWAY_API_KEY || '';

function headers() {
  const h = { 'Content-Type': 'application/json' };
  if (apiKey) {
    h.Authorization = `Bearer ${apiKey}`;
  }
  return h;
}

export default function () {
  const health = http.get(`${baseURL}/health`);
  check(health, { 'health ok': (r) => r.status === 200 });

  const payload = JSON.stringify({
    user_id: 'k6-user',
    tool_name: 'calculator',
    input: { expression: '(12 + 8) / 4' },
  });
  const tool = http.post(`${baseURL}/v1/tools/execute`, payload, { headers: headers() });
  check(tool, {
    'tool status ok': (r) => r.status === 200,
    'tool success': (r) => r.json('data.success') === true,
  });

  sleep(1);
}
