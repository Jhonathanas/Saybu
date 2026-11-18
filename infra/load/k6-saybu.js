// Load test M6-06: 100 VU katalog + 10 checkout/detik.
//
// Jalankan lewat Docker (tidak perlu pasang k6 di host):
//   make load-test
// atau langsung:
//   docker run --rm -i --add-host=host.docker.internal:host-gateway \
//     -e API_BASE=http://host.docker.internal:8081 grafana/k6 run -
//
// Target lokal: p95 < 300 ms untuk katalog.
import http from 'k6/http';
import { check } from 'k6';
import { Counter } from 'k6/metrics';

const API = __ENV.API_BASE || 'http://host.docker.internal:8081';
const businessErrors = new Counter('business_errors');

export const options = {
  scenarios: {
    catalog: {
      executor: 'constant-vus',
      vus: 100,
      duration: __ENV.DURATION || '30s',
      exec: 'browseCatalog',
      tags: { scenario: 'catalog' },
    },
    checkout: {
      executor: 'constant-arrival-rate',
      rate: 10,
      timeUnit: '1s',
      duration: __ENV.DURATION || '30s',
      preAllocatedVUs: 20,
      maxVUs: 60,
      exec: 'checkout',
      tags: { scenario: 'checkout' },
    },
  },
  thresholds: {
    'http_req_duration{scenario:catalog}': ['p(95)<300'],
    'http_req_duration{scenario:checkout}': ['p(95)<500'],
    'http_req_failed{scenario:catalog}': ['rate<0.01'],
  },
};

export function setup() {
  const zones = http.get(`${API}/api/v1/shipping/zones`).json('data');
  const products = http.get(`${API}/api/v1/products?limit=100`).json('data');
  if (!zones || !zones.length || !products || !products.length) {
    throw new Error('setup gagal: zona/produk kosong');
  }
  return {
    zoneId: zones[0].id,
    productIds: products.map((p) => p.id),
  };
}

// IP klien acak per iterasi: load test mensimulasikan banyak klien berbeda
// sehingga batas baca per-IP (120/menit) tidak menutupi kapasitas backend.
function randomIP(prefix) {
  return `${prefix}.${Math.floor(Math.random() * 254) + 1}.${Math.floor(Math.random() * 254) + 1}`;
}

export function browseCatalog() {
  const res = http.get(`${API}/api/v1/products?limit=20`, {
    headers: { 'X-Forwarded-For': randomIP('10.0') },
  });
  check(res, { 'katalog 200': (r) => r.status === 200 });
}

export function checkout(data) {
  const pid = data.productIds[Math.floor(Math.random() * data.productIds.length)];
  const body = JSON.stringify({
    customer: { name: 'Load Test', phone: '081200000001' },
    fulfillment: 'DELIVERY',
    zone_id: data.zoneId,
    address: {
      line: 'Jl. Load 1',
      rt_rw: '01/01',
      kelurahan: 'Sukamaju',
      kecamatan: 'Sukamaju',
      city: 'Bandung',
    },
    payment_method: 'TRANSFER',
    items: [{ product_id: pid, qty: 1 }],
  });

  const res = http.post(`${API}/api/v1/orders`, body, {
    headers: {
      'Content-Type': 'application/json',
      'Idempotency-Key': `${Date.now()}-${Math.random().toString(16).slice(2)}`,
      'X-Forwarded-For': randomIP('10.1'),
    },
  });

  if (res.status === 201) {
    check(res, { 'checkout 201': (r) => r.status === 201 });
  } else if (res.status === 422) {
    // Stok habis / aturan bisnis: bukan kegagalan sistem.
    businessErrors.add(1);
  } else {
    check(res, { 'checkout bukan error server': (r) => r.status < 500 });
  }
}
