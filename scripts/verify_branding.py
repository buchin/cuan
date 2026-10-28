#!/usr/bin/env python3
"""
Automated live verification script for Cuan GitHub Organization and Domain availability.
Queries live GitHub API and official ICANN/Registry RDAP and DNS endpoints.
"""

import json
import socket
import subprocess
import sys
import urllib.error
import urllib.request

GITHUB_CANDIDATES = [
    "cuanstudio",
    "cuan-studio",
    "cuanlabs",
    "cuan-labs",
    "cuanwork",
    "cuan-work",
    "cuanengine",
    "cuan-engine",
]

DOMAIN_CANDIDATES = {
    "cuanstudio.dev": "https://pubapi.registry.google/rdap/domain/cuanstudio.dev",
    "cuan.studio": "https://rdap.identitydigital.services/rdap/domain/cuan.studio",
    "cuan.work": "https://rdap.nic.work/domain/cuan.work",
    "cuanlabs.dev": "https://pubapi.registry.google/rdap/domain/cuanlabs.dev",
}


def verify_github():
    results = []
    for name in GITHUB_CANDIDATES:
        api_url = f"https://api.github.com/users/{name}"
        web_url = f"https://github.com/{name}"
        headers = {"User-Agent": "Cuan-Verifier/1.0"}

        api_status = None
        data = None
        try:
            req = urllib.request.Request(api_url, headers=headers)
            with urllib.request.urlopen(req, timeout=10) as resp:
                api_status = resp.status
                data = json.loads(resp.read().decode("utf-8"))
        except urllib.error.HTTPError as e:
            api_status = e.code
        except Exception as e:
            api_status = str(e)

        web_status = None
        try:
            req = urllib.request.Request(web_url, headers=headers)
            with urllib.request.urlopen(req, timeout=10) as resp:
                web_status = resp.status
        except urllib.error.HTTPError as e:
            web_status = e.code
        except Exception as e:
            web_status = str(e)

        is_available = (api_status == 404) and (web_status == 404)
        entity_type = data.get("type") if data else None
        created_at = data.get("created_at") if data else None

        results.append({
            "name": name,
            "api_status": api_status,
            "web_status": web_status,
            "entity_type": entity_type,
            "created_at": created_at,
            "available": is_available,
        })
    return results


def verify_domains():
    results = []
    for domain, rdap_url in DOMAIN_CANDIDATES.items():
        # DNS Resolution
        ips = []
        ns = []
        dns_status = "NXDOMAIN"
        try:
            dig_a = subprocess.run(
                ["dig", "+short", "A", domain],
                capture_output=True,
                text=True,
                timeout=5,
            )
            ips = [line.strip() for line in dig_a.stdout.strip().splitlines() if line.strip()]
            dig_ns = subprocess.run(
                ["dig", "+short", "NS", domain],
                capture_output=True,
                text=True,
                timeout=5,
            )
            ns = [line.strip() for line in dig_ns.stdout.strip().splitlines() if line.strip()]
            if ips or ns:
                dns_status = "ACTIVE"
        except Exception:
            pass

        # RDAP Lookup
        rdap_status = None
        registrar = None
        reg_date = None
        exp_date = None
        status_flags = []
        try:
            req = urllib.request.Request(
                rdap_url, headers={"User-Agent": "Mozilla/5.0 (Cuan-Verifier/1.0)"}
            )
            with urllib.request.urlopen(req, timeout=10) as resp:
                rdap_status = resp.status
                data = json.loads(resp.read().decode("utf-8"))
                status_flags = data.get("status", [])
                for ev in data.get("events", []):
                    if ev.get("eventAction") == "registration":
                        reg_date = ev.get("eventDate")
                    elif ev.get("eventAction") == "expiration":
                        exp_date = ev.get("eventDate")
                for ent in data.get("entities", []):
                    if "registrar" in ent.get("roles", []):
                        vcard = ent.get("vcardArray", [])
                        if len(vcard) > 1:
                            for item in vcard[1]:
                                if item[0] == "fn":
                                    registrar = item[3]
        except urllib.error.HTTPError as e:
            rdap_status = e.code
        except Exception as e:
            rdap_status = str(e)

        is_available = (rdap_status == 404) and (dns_status == "NXDOMAIN")

        results.append({
            "domain": domain,
            "rdap_status": rdap_status,
            "dns_status": dns_status,
            "ips": ips,
            "nameservers": ns,
            "registrar": registrar,
            "registered_at": reg_date,
            "expires_at": exp_date,
            "status_flags": status_flags,
            "available": is_available,
        })
    return results


def main():
    print("Verifying GitHub Organization Candidates...")
    gh_results = verify_github()
    print("Verifying Domain Candidates...")
    dom_results = verify_domains()

    report = {
        "github_organizations": gh_results,
        "domains": dom_results,
    }
    print(json.dumps(report, indent=2))


if __name__ == "__main__":
    main()
