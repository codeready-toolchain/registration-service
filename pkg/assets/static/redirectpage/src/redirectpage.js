import Keycloak from "keycloak-js";

// URL to get config
const configURL = "/api/v1/authconfig";
const queryString = window.location.search;
const urlParams = new URLSearchParams(queryString);
const link = urlParams.get("link");
const keyword = urlParams.get("keyword");
const selectedId = urlParams.get("selectedId");
const consoleUrlMock = "https://console.redhat.com/openshift/sandbox";
const registrationURL = "/api/v1/signup"; //Url to get User info needed for redirect

const params = {
  keyword,
  selectedId,
};

let idToken;
let keycloak;
let intervalRefRefresh;

export function handleSuccess(data) {
  if (!data || !data.consoleURL || !data.defaultUserNamespace) {
    window.location.href = consoleUrlMock;
    return;
  }

  const baseUrl = `${data.consoleURL}/`;
  const appendedUrl = `${link}/ns/${data.defaultUserNamespace}`;
  const redirectUrl = new URL(baseUrl + appendedUrl);
  let resultUrl;
  if (data.status != "ready") {
    resultUrl = consoleUrlMock;
  } else {
    if (link === "notebookController") {
      resultUrl = `${baseUrl}notebookController/spawner`;
    } else if (link === "dashboard") {
      resultUrl = `${baseUrl}dashboard`;
    } else {
      resultUrl = redirectUrl.toString();
    }
  }
  const url = new URL(resultUrl);
  Object.keys(params).forEach((key) => {
    if (params[key]) {
      url.searchParams.append(key, params[key]);
    }
  });
  window.location.href = url.toString();
}

function handleError(error) {
  window.location.href = consoleUrlMock;
}

function handleUnauthenticated() {
  idToken = null;
  window.location.href = consoleUrlMock;
}

function handleUserInfo() {
  idToken = keycloak.idToken;
}

function refreshToken() {
  keycloak.updateToken(30).catch((error) => {
    console.error("Failed to refresh token:", error);
    handleUnauthenticated();
  });
}

function initializeKeycloak(clientConfig) {
  keycloak = new Keycloak(clientConfig);
  keycloak
    .init({
      onLoad: "check-sso",
      silentCheckSsoRedirectUri:
        window.location.origin + "/silent-check-sso.html",
    })
    .then((authenticated) => {
      if (authenticated) {
        console.log("User is authenticated");
        intervalRefRefresh = setInterval(refreshToken, 15000); //start 15s interval token refresh
        keycloak
          .loadUserInfo()
          .then(handleUserInfo)
          .catch(() => handleError("failed to pull in user data."));
      } else {
        console.log("user not authenticated");
        handleUnauthenticated();
      }
    })
    .catch(() => handleError("Failed to initialize authorization"));
}

// General function to fetch JSON data
export function getJSON(
  method,
  url,
  token,
  callback,
  body = null,
  headers = {}
) {
  var xhr = new XMLHttpRequest();
  xhr.open(method, url, true);
  if (token != null) xhr.setRequestHeader("Authorization", "Bearer " + token);

  Object.entries(headers).forEach(([key, value]) => {
    xhr.setRequestHeader(key, value);
  });

  xhr.responseType = "json";
  xhr.onload = () => {
    var status = xhr.status;
    if (status >= 200 && status < 300) {
      callback(null, xhr.response);
    } else {
      callback(status, xhr.response);
    }
  };
  xhr.send(body ? JSON.stringify(body) : null);
}

function loadAuthLibrary(url, cbSuccess, cbError) {
  const script = document.createElement("script");
  script.setAttribute("src", url);
  script.setAttribute("type", "text/javascript");
  let loaded = false;
  function loadFunction() {
    if (loaded) return;
    loaded = true;
    cbSuccess();
  }
  function errorFunction(error) {
    if (loaded) return;
    cbError(error);
  }
  script.onerror = errorFunction;
  script.onload = loadFunction;
  script.onreadystatechange = loadFunction;
  document.head.appendChild(script);
}

export async function getRedirectData() {
  const xhr = new XMLHttpRequest();

  xhr.open('GET', registrationURL, true);

  xhr.setRequestHeader('Authorization', `Bearer ${idToken}`);
  xhr.onreadystatechange = function () {
    if (xhr.readyState === XMLHttpRequest.DONE) {
      if (xhr.status >= 200 && xhr.status < 300) {
        try {
          const data = JSON.parse(xhr.responseText);
          handleSuccess(data);
        } catch (error) {
          handleError();
        }
      } else {
        handleError();
      }
    }
  }
  xhr.onerror = function () {
    handleError();
  };
  xhr.send();
}

export function redirectUser() {
  if (!idToken) {
    getJSON("GET", configURL, null, (err, data) => {
      if (err) {
        console.error("Error loading client config: ", err);
      } else {
        loadAuthLibrary(
          data["auth-client-library-url"],
          () => {
            const clientConfig = JSON.parse(data["auth-client-config"]);
            initializeKeycloak(clientConfig);
          },
          handleError()
        );
      }
    });
  } else {
    getRedirectData();
  }
}

redirectUser();
