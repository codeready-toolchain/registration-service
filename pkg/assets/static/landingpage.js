
// interval reference
var intervalRef;
var intervalRefRefresh;

// given country code and phone number
var countryCode;
var phoneNumber;

var idToken;

// this is where we load our config from
configURL = '/api/v1/authconfig'

// signup endpoint
signupURL = '/api/v1/signup'

// phone verification endpoint
phoneVerificationURL = '/api/v1/signup/verification'

// activation code endpoint
activationCodeURL = '/api/v1/signup/verification/activation-code'

// loads json data from url, the callback is called with
// error and data, with data the parsed json.
var getJSON = function(method, url, token, callback, body, headers) {
  var xhr = new XMLHttpRequest();
  xhr.open(method, url, true);
  if (token != null)
    xhr.setRequestHeader('Authorization', 'Bearer ' + token)

  if (headers != null) {
    for (const [key, value] of headers.entries()) {
      xhr.setRequestHeader(key, value)
    }
  }

  xhr.responseType = 'json';
  xhr.onload = function() {
    var status = xhr.status;
    if (status >= 200 && status < 300) {
      console.log('getJSON success: ' + url);
      callback(null, xhr.response);
    } else {
      console.log('getJSON error: ' + url);
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
  console.log('hiding all..');
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
  console.log('showing element: ' + elementId);
  document.getElementById(elementId).style.display = 'block';
}

function showError(errorText) {
  hideAll();
  show('state-error');
  document.getElementById('errorStatus').textContent = errorText;
}

// shows a logged in user and its userId.
function showUser(username, userid, originalsub) {
  console.log('showing user..');
  document.getElementById('username').textContent = username;
  document.getElementById('user-loggedin').style.display = 'inline';
  console.log('showing userId..')
  document.getElementById('userid').textContent = userid;
  document.getElementById('userid').style.display = 'inline';
  console.log('showing originalsub..')
  document.getElementById('originalsub').textContent = originalsub;
  document.getElementById('originalsub').style.display = 'inline';
  document.getElementById('login-command').style.display = 'inline';
  document.getElementById('oc-login').style.display = 'none';
  document.getElementById('user-notloggedin').style.display = 'none';
}

// shows login/signup button
function hideUser() {
  console.log('hiding user..');
  document.getElementById('username').textContent = '';
  document.getElementById('user-loggedin').style.display = 'none';
  document.getElementById('userid').style.display = 'none';
  document.getElementById('login-command').style.display = 'none';
  document.getElementById('oc-login').style.display = 'none';
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
  getJSON('GET', signupURL, idToken, function(err, data) {
    if (err != null) {
      console.log('getSignup error..');
      cbError(err, data);
    } else {
      console.log('getSignup successful..');
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
      stopPolling();
      hideAll();
      show('state-initiate-phone-verification');
    } else if (data.status.ready === true) {
      console.log('account is ready..');
      // account is ready to use; stop interval.
      stopPolling();
      consoleURL = data.consoleURL;
      if (consoleURL === undefined) {
        consoleURL = 'n/a'
      } else {
        consoleURL = data.consoleURL + 'topology/ns/' + data.compliantUsername + '-dev';
      }
      cheDashboardURL = data.cheDashboardURL;
      proxyURL = 'oc login --token='+idToken+' --server=' +data.proxyURL;
      document.getElementById('expandable-not-expanded-readonly-text-input').value = proxyURL;
      if (cheDashboardURL === undefined) {
        cheDashboardURL = 'n/a'
      }
      console.log('showing dashboard..');
      hideAll();
      show('dashboard');
      document.getElementById('stateConsole').href = consoleURL;
      document.getElementById('cheDashboard').href = cheDashboardURL;
    } else if (data.status.ready === false && data.status.reason === 'Provisioning') {
      console.log('account is provisioning..');
      // account is provisioning; start polling.
      hideAll();
      show('state-waiting-for-provisioning')
      startPolling();
    } else {
      console.log('account in unknown state, start polling..');
      // account is in an unknown state, display pending approval; start polling.
      hideAll();
      show('state-waiting-for-approval')
      startPolling();
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
        stopPolling();
        hideAll();
        show('state-getstarted');
      }
    } else if (err === 401) {
      console.log('error 401');
      // user is unauthorized, show login/signup view; stop interval.
      stopPolling();
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

function stopPolling() {
  console.log('stop polling..');
  if (intervalRef) {
    clearInterval(intervalRef);
    intervalRef = undefined;
  }
}

function startPolling() {
  console.log('start polling..');
  if (!intervalRef) {
    intervalRef = setInterval(updateSignupState, 1000);
  }
}

function refreshToken() {
  // if the token is still valid for the next 30 sec, it is not refreshed.
  console.log('check refreshing token..');
  keycloak.updateToken(30)
    .then(function(refreshed) {
      console.log('token refresh result: ' + refreshed);
    }).catch(function() {
      console.log('failed to refresh the token, or the session has expired');
    });
}

function login() {
  // User clicked on Get Started. We can enable autoSignup after successful login now.
  window.sessionStorage.setItem('autoSignup', 'true');
  keycloak.login()
}

// start signup process.
function signup() {
  grecaptcha.enterprise.ready(async () => {
    recaptchaToken = await grecaptcha.enterprise.execute('6LdL7aMlAAAAALvuuAZWjwlOLRKMCIrWjOpv-U3G', {action: 'SIGNUP'});
    var headers = new Map();
    headers.set("Recaptcha-Token", recaptchaToken)
    getJSON('POST', signupURL, idToken, function(err, data) {
      if (err != null) {
        showError(JSON.stringify(data, null, 2));
      } else {
        hideAll();
        show('state-waiting-for-approval');
      }
    }, null, headers);
  });
  startPolling();
}

function submitActivationCode() {
  code = document.getElementById("activationcode").value;
  // check validity
  let codeValid = /^[a-z0-9]{5}$/.test(code);
  if (!codeValid) {
    showError('Activation code is invalid, please check your input.');
    show('state-initiate-phone-verification');
  } else {
    getJSON('POST', activationCodeURL, idToken, function(err, data) {
      if (err != null) {
        showError('Activation code is not valid. Please try again later.');
      } else {
        // code verification success, refresh signup state
        updateSignupState();
      }
    }, {
      code: code,
    }, );
  }
}

function initiatePhoneVerification() {
  countryCode = document.getElementById("phone-countrycode").value;
  phoneNumber = document.getElementById("phone-phonenumber").value;
  // check validity
  let phoneValid = /^[(]?[0-9]+[)]?[-\s\.]?[0-9]+[-\s\.\/0-9]*$/im.test(phoneNumber);
  let countryCodeValid = /^[\+]?[0-9]+$/.test(countryCode);
  if (!phoneValid || !countryCodeValid) {
    showError('Phone or country code invalid, please check your input.');
    show('state-initiate-phone-verification');
  } else {
    getJSON('PUT', phoneVerificationURL, idToken, function(err, data) {
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
    getJSON('GET', phoneVerificationURL + '/' + verificationCode, idToken, function(err, data) {
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
    getJSON('PUT', phoneVerificationURL, idToken, function(err, data) {
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

function restartPhoneVerification() {
  console.log('updating phone number..');
  stopPolling();
  hideAll();
  show('state-initiate-phone-verification');
}

function termsAgreed(cb) {
  if (cb.checked) {
    document.getElementById('loginbutton').classList.remove('getstartedbutton-disabled');
    document.getElementById('loginbutton').classList.add('getstartedbutton-enabled');  
  } else {
    document.getElementById('loginbutton').classList.add('getstartedbutton-disabled');
    document.getElementById('loginbutton').classList.remove('getstartedbutton-enabled');  
  }
}

function showLoginCommand() {
  document.getElementById('login-command').style.display = 'none'
  document.getElementById('oc-login').style.display = 'inline'
}

function copyCommand() {
  var inputText = document.getElementById('expandable-not-expanded-readonly-text-input');
  navigator.clipboard.writeText(inputText.value);
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
      keycloak = new Keycloak(clientConfig);
      keycloak.init({
        onLoad: 'check-sso',
        silentCheckSsoRedirectUri: window.location.origin + '/silent-check-sso.html',
      }).then(function(authenticated) {
        if (authenticated == true) {
          console.log('user is authenticated');
          // start 15s interval token refresh.
          intervalRefRefresh = setInterval(refreshToken, 15000);
          keycloak.loadUserInfo()
            .then(function(data) {
              console.log('retrieved user info..');
              idToken = keycloak.idToken
              showUser(data.preferred_username, data.sub, data.original_sub)
              // now check the signup state of the user.
              updateSignupState();
            })
            .catch(function() {
              console.log('Failed to pull in user data');
              showError('Failed to pull in user data.');
            });
        } else {
          console.log('user not authenticated');
          hideUser();
          hideAll();
          idToken = null
          show('state-getstarted');
        }
      }).catch(function() {
        console.log('Failed to initialize authorization');
        showError('Failed to initialize authorization.');
      });
    }, function(err) {
      console.log('error loading client library' + err);
      showError(err);
    });
  }
});
