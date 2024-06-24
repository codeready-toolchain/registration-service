const queryString = window.location.search;
const urlParams = new URLSearchParams(queryString);
const link = urlParams.get("link");
const keyword = urlParams.get("keyword");
const selectedId = urlParams.get("selectedId");
const consoleUrl = "https://console.redhat.com/openshift/sandbox";
const baseUrl = `https://${data.consoleURL}/`;
const appendedUrl = `${link}/ns/${data.defaultUserNamespace}`;
const params = {
  keyword,
  selectedId,
};
const redirectUrl = new URL(baseUrl + appendedUrl);

Object.keys(params).forEach((key) => {
  if (params[key]) {
    urlParams.searchParams.append(key, params[key]);
  }
});

function handleSuccess(data) {
  if (data.status != "ready") {
    window.location.href = "https://console.redhat.com/openshift/sandbox";
  } else {
    window.location.href =
      link === "notebookController"
        ? `${baseUrl}notebookController/spawner`
        : link === "dashboard"
        ? `${baseUrl}dashboard`
        : redirectUrl.toString();
  }
}

function handleError() {
  window.location.href = "https://console.redhat.com/openshift/sandbox";
}

if (keycloak) {
  fetch(
    "registration-service-toolchain-host-operator.apps.sandbox.x8i5.p1.openshiftapps.com/api/v1/signup",
    {
      method: "GET",
      headers: {
        Authorization: "Bearer" + keycloak.token,
      },
    }
  )
    .then((response) => response.json())
    .then((data) => handleSuccess(data))
    .catch((error) => handleError(error));
} else {
    login();
}
