# Sentinel DDoS Lab — Safe Local Simulation

A local Go web app for demonstrating service overload, gradual degradation, and a defensive rate-limiting control. It does not generate real attack traffic or target external systems.

## Run

```bash
go run .
```

Open these pages in separate tabs:
- http://localhost:8080/monitor — live monitoring and defensive controls
- http://localhost:8080/attacker — gradual traffic-load simulator
- http://localhost:8080/user — normal streaming-user view

## Demo flow

1. Open all three pages.
2. Start the simulation in the attacker page. Traffic ramps up gradually (50 simulated requests/second per second). The dashboard shows the service moving through healthy, degraded, and offline states.
3. In the monitoring page, enable **Per-source Token Bucket Rate Limiting**. The simulated attacking source is capped at 20 requests/second while legitimate user traffic is preserved. Watch effective incoming traffic and availability recover in real time.
4. Stop the simulation or reset the lab.

## Technical notes

- Real-time updates use Server-Sent Events (SSE); no page refresh is needed.
- Traffic, rate limiting, accepted/dropped/blocked request counters, CPU load, and availability are simulated mathematically. No request flood, attack workers, or external target traffic is created.
- The mitigation demonstrates a real defensive pattern (per-source rate limiting / token bucket) but is modeled in the lab; it is not a production firewall or network-layer DDoS protection. A real deployment should enforce limits at a reverse proxy or API gateway and correctly identify client IPs behind trusted proxies.
- App memory is read from Go runtime statistics. CPU is a simulated load indicator, not host CPU utilization.
# ddos
