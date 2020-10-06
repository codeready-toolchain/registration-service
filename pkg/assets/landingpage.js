
// interval reference
var intervalRef;

// given country code and phone number
var countryCode;
var phoneNumber;

// this is where we load our config from
configURL = '/api/v1/authconfig'

// signup endpoint
signupURL = '/api/v1/signup'

// phone verification endpoint
phoneVerificationURL = '/api/v1/signup/verification'

// loads json data from url, the callback is called with
// error and data, with data the parsed json.
var getJSON = function(method, url, token, callback, body) {
  var xhr = new XMLHttpRequest();
  xhr.open(method, url, true);
  if (token != null)
    xhr.setRequestHeader('Authorization', 'Bearer ' + token)
  xhr.responseType = 'json';
  xhr.onload = function() {
    var status = xhr.status;
    if (status >= 200 && status < 300) {
      callback(null, xhr.response);
    } else {
      callback(status, xhr.response);
    }
  };
  if (body)
    xhr.send(JSON.stringify(body));
  else
    xhr.send();
};

// hides all state content.
function hideAll() {
  document.getElementById('state-waiting-for-provisioning').style.display = 'none';
  document.getElementById('state-waiting-for-approval').style.display = 'none';
  document.getElementById('state-provisioned').style.display = 'none';
  document.getElementById('state-getstarted').style.display = 'none';
  document.getElementById('state-error').style.display = 'none';
  document.getElementById('dashboard').style.display = 'none';
  document.getElementById('state-initiate-phone-verification').style.display = 'none';
  document.getElementById('state-complete-phone-verification').style.display = 'none';
}

// shows state content. Given Id needs to be one of
// state-waiting-for-provisioning, state-waiting-for-approval,
// state-provisioned, state-getstarted, dashboard, state-error
function show(elementId) {
  document.getElementById(elementId).style.display = 'block';
}

function showError(errorText) {
  hideAll();
  show('state-error');
  document.getElementById('errorStatus').textContent = errorText;
}

// shows a logged in user.
function showUser(username) {
  document.getElementById('username').textContent = username;
  document.getElementById('user-loggedin').style.display = 'inline';
  document.getElementById('user-notloggedin').style.display = 'none';
}

// shows login/signup button
function hideUser() {
  document.getElementById('username').textContent = '';
  document.getElementById('user-loggedin').style.display = 'none';
  document.getElementById('user-notloggedin').style.display = 'inline';
}

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
  document.getElementsByTagName('head')[0].appendChild(script);
}
      
// gets the signup state once.
function getSignupState(cbSuccess, cbError) {
  getJSON('GET', signupURL, keycloak.idToken, function(err, data) {
    if (err != null) {
      cbError(err, data);
    } else {
      cbSuccess(data);
    }
  })
}

// updates the signup state.
function updateSignupState() {
  console.log('updating signup state..');
  getSignupState(function(data) {
    if (data.status.ready === false && data.status.verificationRequired) {
      console.log('verification required..');
      hideAll();
      show('state-initiate-phone-verification');
    } else if (data.status.ready === true) {
      console.log('account is ready..');
      // account is ready to use; stop interval.
      if (intervalRef)
        clearInterval(intervalRef);
      consoleURL = data.consoleURL;
      if (consoleURL === undefined) {
        consoleURL = 'n/a'
      } else {
        consoleURL = data.consoleURL + 'topology/ns/' + data.compliantUsername + '-dev';
      }
      cheDashboardURL = data.cheDashboardURL;
      if (cheDashboardURL === undefined) {
        cheDashboardURL = 'n/a'
      }
      console.log('showing dashboard..');
      hideAll();
      show('dashboard')
      document.getElementById('stateConsole').href = consoleURL;
      document.getElementById('cheDashboard').href = cheDashboardURL;
    } else if (data.status.ready === false && data.status.reason === 'Provisioning') {
      console.log('account is provisioning..');
      // account is provisioning; start polling.
      hideAll();
      show('state-waiting-for-provisioning')
      if (!intervalRef) {
        // only start if there is not already a polling running.
        intervalRef = setInterval(updateSignupState, 1000);
      }
    } else {
      console.log('account in unknown state, start polling..');
      // account is in an unknown state, display pending approval; start polling.
      hideAll();
      show('state-waiting-for-approval')
      if (!intervalRef) {
        // only start if there is not already a polling running.
        intervalRef = setInterval(updateSignupState, 1000);
      }
    }
  }, function(err, data) {
    if (err === 404) {
      console.log('error 404');
      // signup does not exist, but user is authorized, check if we can start signup process.
      if ('true' === window.sessionStorage.getItem('autoSignup')) {
        console.log('autoSignup is true..');
        // user has explicitly requested a signup
        window.sessionStorage.removeItem('autoSignup');
        signup();
      } else {
        console.log('autoSignup is false..');
        // we still need to show GetStarted button even if the user is logged-in to SSO to avoid auto-signup without users clicking on Get Started button
        clearInterval(intervalRef);
        hideAll();
        show('state-getstarted');
      }
    } else if (err === 401) {
      console.log('error 401');
      // user is unauthorized, show login/signup view; stop interval.
      clearInterval(intervalRef);
      hideUser();
      hideAll();
      show('state-getstarted');
      show('state-error');
      if(data != null && data.error != null){
        document.getElementById('errorStatus').textContent = data.error;
      }
    } else {
      // other error, show error box.
      showError(err);
    }
  })
}

function login() {
  // User clicked on Get Started. We can enable autoSignup after successful login now.
  window.sessionStorage.setItem('autoSignup', 'true');
  keycloak.login()
}

// start signup process.
function signup() {
  getJSON('POST', signupURL, keycloak.idToken, function(err, data) {
    if (err != null) {
      showError(JSON.stringify(data, null, 2));
    } else {
      hideAll();
      show('state-waiting-for-approval');
    }
  });
  intervalRef = setInterval(updateSignupState, 1000);
}

function initiatePhoneVerification() {
  countryCode = document.getElementById("phone-countrycode").value;
  phoneNumber = document.getElementById("phone-phonenumber").value;
  // check validity
  let phoneValid = /^[(]?[0-9]{3}[)]?[-\s\.]?[0-9]{3}[-\s\.]?[0-9]{4,6}$/im.test(phoneNumber);
  let countryCodeValid = /^[\+]?[0-9]{1,4}$/.test(countryCode);
  if (!phoneValid || !countryCodeValid) {
    showError('phone or country code invalid, please check your input.');
    show('state-initiate-phone-verification');
  } else {
    getJSON('PUT', phoneVerificationURL, keycloak.idToken, function(err, data) {
      if (err != null) {
        showError('Error while sending verification code. Please try again later.');
      } else {
        hideAll();
        show('state-complete-phone-verification');
      }
    }, {
      country_code: countryCode,
      phone_number: phoneNumber
    });
  }
}

function completePhoneVerification() {
  let verificationCode = document.getElementById("phone-verificationcode").value;
  let verificationCodeValid = /^[\+]?[a-z0-9]{6}$/im.test(verificationCode);
  if (!verificationCodeValid) {
    showError('verification code has the wrong format, please check your input.');
    show('state-complete-phone-verification');
  } else {
    getJSON('GET', phoneVerificationURL + '/' + verificationCode, keycloak.idToken, function(err, data) {
      if (err != null) {
        showError('Error while sending verification code. Please try again later.');
      } else {
        // code verification success, refresh signup state
        updateSignupState();
      }
    });
  }
}

function resendPhoneVerification() {
    getJSON('PUT', phoneVerificationURL, keycloak.idToken, function(err, data) {
      if (err != null) {
        showError('Error while sending verification code. Please try again later.');
      } 
    }, {
      country_code: countryCode,
      phone_number: phoneNumber
    });
    document.getElementById('phone-verificationcode-resend-status').style.display = 'inline';
    setTimeout(function() {
      document.getElementById('phone-verificationcode-resend-status').style.display = 'none';
    }, 2000);
}

// main operation, load config, load client, run client
getJSON('GET', configURL, null, function(err, data) {
  if (err !== null) {
    console.log('error loading client config' + err);
    showError(err);
  } else {
    loadAuthLibrary(data['auth-client-library-url'], function() {
      console.log('client library load success!')
      var clientConfig = JSON.parse(data['auth-client-config']);
      console.log('using client configuration: ' + JSON.stringify(clientConfig))
      keycloak = Keycloak(clientConfig);
      keycloak.init({
        onLoad: 'check-sso',
        silentCheckSsoRedirectUri: window.location.origin,
      }).success(function(authenticated) {
        if (authenticated == true) {
          console.log('user is authenticated');
          keycloak.loadUserInfo()
            .success(function(data) {
              console.log('retrieved user info..');
              showUser(data.preferred_username)
              // now check the signup state of the user.
              updateSignupState();
            })
            .error(function() {
              console.log('Failed to pull in user data');
              showError('Failed to pull in user data.');  
            });
        } else {
          hideUser();
          hideAll();
          show('state-getstarted');
        }
      }).error(function() {
        console.log('Failed to initialize authorization');
        showError('Failed to initialize authorization.');
      });
    }, function(err) {
      console.log('error loading client library' + err);
      showError(err);
    });
  }
});
