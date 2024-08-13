import {
  handleSuccess,
  getJSON,
  getRedirectData,
  redirectUser,
} from "./redirectpage";

//Mock dependencies
global.fetch = jest.fn();
const mockKeycloak = {
  init: jest.fn(),
  loadUserInfo: jest.fn(),
  idToken: "mock-token",
};
global.Keycloak = jest.fn(() => mockKeycloak);

const mockXHR = {
  open: jest.fn(),
  setRequestHeader: jest.fn(),
  send: jest.fn(),
  onload: jest.fn(),
  status: 200,
  response: { key: "value" },
};

//Mock XMLHttpRequest
global.XMLHttpRequest = jest.fn(() => mockXHR);

global.refreshToken = jest.fn();
global.updateSignupState = jest.fn();
global.consoleUrlMock = "https://example.com";

describe("redirect user functions", () => {
  describe("handleSuccess", () => {

    beforeEach(() => {
      //Mock window.location
      delete window.location;
    });

    it("should redirect to consoleUrl when status is not ready", () => {
      const dataMock = {
        status: "not_ready",
        consoleURL: "https://example.com/",
        defaultUserNamespace: "user123",
      };
      window.location = {href: 'https:example.com'};
      handleSuccess(dataMock);
      expect(window.location.href).toBe(
        "https://console.redhat.com/openshift/sandbox"
      );
    });
    it("should construct URL with parameters and redirect correctly", () => {
      const dataMock = {
        status: "ready",
        consoleURL: "https://data.example.com",
        defaultUserNamespace: "user123",
      };

      //Set the link parameter and mock search params
      window.location = {href: 'https://example.com'};

      handleSuccess(dataMock);
      expect(window.location.href).toBe(
        "https://data.example.com/null/ns/user123"
      );
    });
  });

  describe("getJSON", () => {
    it("should make a GET request and return JSON response", () => {
      const callback = jest.fn();

      getJSON("GET", "https://example.com", null, callback);

      //simulate XHR success
      mockXHR.onload();
      expect(mockXHR.open).toHaveBeenCalledWith(
        "GET",
        "https://example.com",
        true
      );
      expect(mockXHR.send).toHaveBeenCalled();
      expect(callback).toHaveBeenCalledWith(null, { key: "value" });
      jest.clearAllMocks();
    });

    it("should handle error response", () => {
      const callback = jest.fn();
      mockXHR.status = 400;

      getJSON("GET", "https://example.com", null, callback);

      //simulate XHR success
      mockXHR.onload();

      expect(callback).toHaveBeenCalledWith(400, { key: "value" });
      jest.clearAllMocks();
    });
  });

  describe("getRedirectData", () => {
    it("should fetch redirect data and handle success", () => {
      const handleSuccessMock = jest.fn();
      const handleError = jest.fn();
      getRedirectData();

      //simulate XHR success
      mockXHR.onload();

      expect(mockXHR.send).toHaveBeenCalled();
      setTimeout(() => {
        expect(handleSuccessMock).toHaveBeenCalled();
        expect(handleError).not.toHaveBeenCalled();
      }, 0);
      jest.clearAllMocks();
    });

    it("should handle fetch error", () => {
      mockXHR.status = 400;

      getRedirectData();

      //simulate XHR success
      mockXHR.onload();

      setTimeout(() => {
        expect(handleError).toHaveBeenCalled();
      }, 0);
      jest.clearAllMocks();
    });
  });

  describe("redirectUser", () => {
    it("should call getRedirectData if idToken exists", () => {
      global.idToken = "mock-token";
      const getRedirectDataMock = jest.fn();

      redirectUser();
      //simulate XHR success
      mockXHR.onload();
      setTimeout(() => {
        expect(getRedirectDataMock).toHaveBeenCalled();
      }, 0);
      jest.clearAllMocks();
    });

    it("should redirect to sso if idToken does not exist", () => {
      global.idToken = null;
      const getJSONMock = jest.fn((method, url, token, callback) =>
        callback(null, {
          "auth-client-library-url": "mock-url",
          "auth-client-config": "{}",
        })
      );
      global.getJSON = getJSONMock;

      redirectUser();

      //simulate XHR success
      mockXHR.onload();
      setTimeout(() => {
        expect(getJSONMock).toHaveBeenCalledWith(
          "GET",
          expect.any(String),
          null,
          expect.any(Function)
        );
      }, 0);

      jest.clearAllMocks();
    });
  });
});
