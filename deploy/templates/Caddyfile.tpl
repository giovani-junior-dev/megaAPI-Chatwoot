# Caddyfile — generated from deploy/templates/Caddyfile.tpl by install.sh
# Routes: ${DOMAIN} -> chatwoot rails (3000); /admin and /v1 -> bridge (8080)

{
    email ${EMAIL}
}

${DOMAIN} {
    encode zstd gzip

    header {
        Strict-Transport-Security "max-age=63072000; includeSubDomains; preload"
        X-Frame-Options "DENY"
        Content-Security-Policy "default-src 'self'; script-src 'self' 'unsafe-eval' 'unsafe-inline' https://unpkg.com https://cdn.tailwindcss.com; style-src 'self' 'unsafe-inline' https://cdn.tailwindcss.com; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'"
        X-Content-Type-Options "nosniff"
        Referrer-Policy "strict-origin-when-cross-origin"
        Permissions-Policy "geolocation=(), microphone=(), camera=()"
        -Server
    }

    handle_path /admin/* {
        reverse_proxy bridge:8080
    }

    handle /v1/* {
        reverse_proxy bridge:8080
    }

    handle /healthz {
        reverse_proxy bridge:8080
    }

    handle /readyz {
        reverse_proxy bridge:8080
    }

    handle {
        reverse_proxy rails:3000
    }
}
