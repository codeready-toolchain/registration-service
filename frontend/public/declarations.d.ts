declare interface KeyCloak {
    authenticated: boolean,
    init: Function,
    login: Function,
    logout: Function,
    token: string
}

declare interface Window {
    clientConfig: object,
    keycloak: KeyCloak,
    Keycloak: Function
}