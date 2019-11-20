(function (window) {
    // time elapsed, used for the state poll
    var elapsed = 0

    // var host = "https://registration-service-host-operator-1573325166.192.168.64.3.nip.io"
    // this is where we load our config from
    var configURL = "/api/v1/authconfig"

    // loads json data from url, the callback is called with
    // error and data, with data the parsed json.
    var getJSON = function (url, token, callback) {
        var xhr = new XMLHttpRequest();
        xhr.open('GET', url, true);
        if (token != null)
            xhr.setRequestHeader("Authorization", "Bearer " + token)
        xhr.responseType = 'json';
        xhr.onload = function () {
            var status = xhr.status;
            if (status === 200) {
                callback(null, xhr.response);
            } else {
                callback(status, xhr.response);
            }
        };
        xhr.send();
    };

    // this loads the js library at location 'url' dynamically and
    // calls 'cbSuccess' when the library was loaded successfully
    // and 'cbError' when there was an error loading the library.
    function loadAuthLibrary(url, cbSuccess, cbError) {
        var script = document.createElement('script');
        script.setAttribute('src', url);
        script.setAttribute('type', 'text/javascript');
        var loaded = false;
        var loadFunction = function () {
            if (loaded) return;
            loaded = true;
            cbSuccess();
        };
        var errorFunction = function (error) {
            if (loaded) return;
            cbError(error)
        };
        script.onerror = errorFunction;
        script.onload = loadFunction;
        script.onreadystatechange = loadFunction;
        document.getElementsByTagName("head")[0].appendChild(script);
    };

    function updateSignupState() {
        elapsed++;
        getJSON("/api/v1/signup", window.keycloak.idToken, function (err, data) {
            if (err != null) {
                console.error("Error status", err);
                // document.getElementById('errorStatus').textContent = error;
            } else {
                console.log("serviceResp ", JSON.stringify(data, null, 2));
                // document.getElementById('stateResp').textContent = JSON.stringify(data, null, 2);
                // document.getElementById('stateElapsed').textContent = elapsed;
            }
        })
    }

    // main operation, load config, load client, run client
    getJSON(configURL, null, function (err, data) {
        if (err !== null || data === null) {
            console.log('error loading client config' + err);
            // document.getElementById('errorStatus').textContent = error;
        } else {
            loadAuthLibrary(data['auth-client-library-url'], function () {
                console.log("client library load success!")
                window.clientConfig = JSON.parse(data['auth-client-config']);
                console.log("using client configuration: " + JSON.stringify(window.clientConfig))
                window.keycloak = window.Keycloak(window.clientConfig);
                window.keycloak.init().then(function (authenticated) {
                    if (authenticated === true) {
                        window.keycloak.loadUserInfo().success(function (data) {
                            console.log("User info", data);
                            // document.getElementById('loginStatus').textContent = "logged in as user " + data.preferred_username;
                            // document.getElementById('jwtToken').textContent = JSON.stringify(keycloak.idTokenParsed, null, 2);
                            // do an authenticated request
                            // note, this only works if testingmode is set to true!
                            getJSON("/api/v1/auth_test", window.keycloak.idToken, function (err, data) {
                                if (err != null) {
                                    console.error("Error status", err);
                                    // document.getElementById('errorStatus').textContent = error;
                                } else {
                                    console.log("serviceResp ", JSON.stringify(data, null, 2));
                                    // document.getElementById('serviceResp').textContent = JSON.stringify(data, null, 2);
                                }
                            })
                        });
                        updateSignupState();
                    } else {
                        console.log("Not logged in!!");                         
                        // document.getElementById('loginStatus').textContent = "not logged in";
                    }
                }).error(function () {
                    console.log('failed to initialize');
                });
            }, function (error) {
                console.log('error loading client library' + error);
                // document.getElementById('errorStatus').textContent = error;
            });
        }
    });
})(window);