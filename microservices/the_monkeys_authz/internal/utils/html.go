package utils

import (
	"github.com/sirupsen/logrus"
	"github.com/the-monkeys/the_monkeys/config"
)

var Address string

func init() {
	cfg, err := config.GetConfig()
	if err != nil {
		logrus.Errorf("cannot load the config: %v", err)
	}
	Address = cfg.Authentication.EmailVerificationAddr
	logrus.Infof("✅ the email verification address is: %v", Address)
}

func ResetPasswordTemplate(firstName, lastName, secret string, username string) string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>Welcome to Monkeys</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            background-color: #fff4ed;
            margin: 0;
            padding: 20px;
            display: flex;
            flex-direction: column;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
        }

        h1 {
            color: #101010;
            margin-bottom: 20px;
            text-align: center;
            font-family: serif;
            font-size: 36px;
        }

        p {
            color: #000;
            line-height: 1.6;
            margin-bottom: 20px;
            font-weight: 500;
            font-size: 16px;
        }

        a {
            text-decoration: none;
            display: inline-block;
        }

        .btn {
            background-color: #ff462e;
            color: #f2f2f3;
            border: none;
            padding: 15px 30px;
            text-transform: uppercase;
            font-weight: 600;
            font-size: 16px;
            border-radius: 5px;
            cursor: pointer;
        }

        .btn:hover {
            background-color: #f7381f;
        }

        .footer {
            margin-top: 40px;
            display: flex;
            flex-direction: column;
            align-items: center;
        }

        .footer_icon {
            opacity: 0.75;
        }

        .footer_icon:hover {
            opacity: 1;
        }
    </style>
</head>

<body>
    <main class="container">

        <h1 style="margin-bottom: 100px; cursor: default">
            Welcome to <span style="color: #ff462e">Monkeys</span>
        </h1>

        <p>Hello ` + firstName + ` ` + lastName + `,</p>

        <p>
            We noticed you requested a password reset for your The Monkeys account. No worries, we've got you covered!
        </p>

        <p>To create a new, secure password, simply click the button below:</p>

        <div style="margin: 20px 0; display: flex; justify-content: center">
            <a href="` + Address + `/auth/reset-password?user=` + username + `&evpw=` + secret + `" target="_blank">
                <button class="btn">Verify Email Address</button>
            </a>
        </div>

        <p>
            <b>Alternatively</b>, you can copy and paste the link below into your web browser:
        </p>

        <a href="` + Address + `/auth/reset-password?user=` + username + `&evpw=` + secret + `" target="_blank">
            ` + Address + `/auth/reset-password?user=` + username + `&evpw=` + secret + `
        </a>

        <p style="color: #ed3232">
            This link will expire in 1 hours for your security. If you don't reset your password within that time, you can request a new link anytime.
        </p>

        <p>
            Once your password is reset, you can dive in and start using <span style="font-weight: bold">The Monkeys</span> again. If you have any trouble verifying your email, please feel free to contact our support team at <b>mail.themonkeys.life@gmail.com</b>. We're happy to help.
        </p>

        <p>We always welcome to the community,</p>

        <p>Thanks,<br />The Monkeys Team</p>

        <footer class="footer">
            <div style="display: flex; gap: 10px">
                <a href="https://github.com/the-monkeys" target="_blank" aria-label="GitHub">
                    <div style="height: 24px; width: 24px" class="footer_icon">
                        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#101010">
                            <!-- SVG Path Here -->
                        </svg>
                    </div>
                </a>
                <!-- Add other social media links similarly -->
            </div>
        </footer>
    </main>
</body>
</html>
`
}

// func ResetPasswordTemplate(firstName, LastName, secret string, username string) string {
// 	return `<!DOCTYPE html PUBLIC>
// 	<html xmlns="http://www.w3.org/1999/xhtml">
// 	  <head>
// 		<meta name="viewport" content="width=device-width, initial-scale=1.0" />
// 		<meta name="x-apple-disable-message-reformatting" />
// 		<meta http-equiv="Content-Type" content="text/html; charset=UTF-8" />
// 		<meta name="color-scheme" content="light dark" />
// 		<meta name="supported-color-schemes" content="light dark" />
// 		<title></title>
// 		<style type="text/css" rel="stylesheet" media="all">
// 		/* Base ------------------------------ */

// 		@import url("https://fonts.googleapis.com/css?family=Nunito+Sans:400,700&display=swap");
// 		body {
// 		  width: 100% !important;
// 		  height: 100%;
// 		  margin: 0;
// 		  -webkit-text-size-adjust: none;
// 		}

// 		a {
// 		  color: #3869D4;
// 		}

// 		a img {
// 		  border: none;
// 		}

// 		td {
// 		  word-break: break-word;
// 		}

// 		.preheader {
// 		  display: none !important;
// 		  visibility: hidden;
// 		  mso-hide: all;
// 		  font-size: 1px;
// 		  line-height: 1px;
// 		  max-height: 0;
// 		  max-width: 0;
// 		  opacity: 0;
// 		  overflow: hidden;
// 		}
// 		/* Type ------------------------------ */

// 		body,
// 		td,
// 		th {
// 		  font-family: "Nunito Sans", Helvetica, Arial, sans-serif;
// 		}

// 		h1 {
// 		  margin-top: 0;
// 		  color: #333333;
// 		  font-size: 22px;
// 		  font-weight: bold;
// 		  text-align: left;
// 		}

// 		h2 {
// 		  margin-top: 0;
// 		  color: #333333;
// 		  font-size: 16px;
// 		  font-weight: bold;
// 		  text-align: left;
// 		}

// 		h3 {
// 		  margin-top: 0;
// 		  color: #333333;
// 		  font-size: 14px;
// 		  font-weight: bold;
// 		  text-align: left;
// 		}

// 		td,
// 		th {
// 		  font-size: 16px;
// 		}

// 		p,
// 		ul,
// 		ol,
// 		blockquote {
// 		  margin: .4em 0 1.1875em;
// 		  font-size: 16px;
// 		  line-height: 1.625;
// 		}

// 		p.sub {
// 		  font-size: 13px;
// 		}
// 		/* Utilities ------------------------------ */

// 		.align-right {
// 		  text-align: right;
// 		}

// 		.align-left {
// 		  text-align: left;
// 		}

// 		.align-center {
// 		  text-align: center;
// 		}

// 		.u-margin-bottom-none {
// 		  margin-bottom: 0;
// 		}
// 		/* Buttons ------------------------------ */

// 		.button {
// 		  background-color: #3869D4;
// 		  border-top: 10px solid #3869D4;
// 		  border-right: 18px solid #3869D4;
// 		  border-bottom: 10px solid #3869D4;
// 		  border-left: 18px solid #3869D4;
// 		  display: inline-block;
// 		  color: #FFF;
// 		  text-decoration: none;
// 		  border-radius: 3px;
// 		  box-shadow: 0 2px 3px rgba(0, 0, 0, 0.16);
// 		  -webkit-text-size-adjust: none;
// 		  box-sizing: border-box;
// 		}

// 		.button--green {
// 		  background-color: #22BC66;
// 		  border-top: 10px solid #22BC66;
// 		  border-right: 18px solid #22BC66;
// 		  border-bottom: 10px solid #22BC66;
// 		  border-left: 18px solid #22BC66;
// 		}

// 		.button--red {
// 		  background-color: #FF6136;
// 		  border-top: 10px solid #FF6136;
// 		  border-right: 18px solid #FF6136;
// 		  border-bottom: 10px solid #FF6136;
// 		  border-left: 18px solid #FF6136;
// 		}

// 		@media only screen and (max-width: 500px) {
// 		  .button {
// 			width: 100% !important;
// 			text-align: center !important;
// 		  }
// 		}
// 		/* Attribute list ------------------------------ */

// 		.attributes {
// 		  margin: 0 0 21px;
// 		}

// 		.attributes_content {
// 		  background-color: #F4F4F7;
// 		  padding: 16px;
// 		}

// 		.attributes_item {
// 		  padding: 0;
// 		}
// 		/* Related Items ------------------------------ */

// 		.related {
// 		  width: 100%;
// 		  margin: 0;
// 		  padding: 25px 0 0 0;
// 		  -premailer-width: 100%;
// 		  -premailer-cellpadding: 0;
// 		  -premailer-cellspacing: 0;
// 		}

// 		.related_item {
// 		  padding: 10px 0;
// 		  color: #CBCCCF;
// 		  font-size: 15px;
// 		  line-height: 18px;
// 		}

// 		.related_item-title {
// 		  display: block;
// 		  margin: .5em 0 0;
// 		}

// 		.related_item-thumb {
// 		  display: block;
// 		  padding-bottom: 10px;
// 		}

// 		.related_heading {
// 		  border-top: 1px solid #CBCCCF;
// 		  text-align: center;
// 		  padding: 25px 0 10px;
// 		}
// 		/* Discount Code ------------------------------ */

// 		.discount {
// 		  width: 100%;
// 		  margin: 0;
// 		  padding: 24px;
// 		  -premailer-width: 100%;
// 		  -premailer-cellpadding: 0;
// 		  -premailer-cellspacing: 0;
// 		  background-color: #F4F4F7;
// 		  border: 2px dashed #CBCCCF;
// 		}

// 		.discount_heading {
// 		  text-align: center;
// 		}

// 		.discount_body {
// 		  text-align: center;
// 		  font-size: 15px;
// 		}
// 		/* Social Icons ------------------------------ */

// 		.social {
// 		  width: auto;
// 		}

// 		.social td {
// 		  padding: 0;
// 		  width: auto;
// 		}

// 		.social_icon {
// 		  height: 20px;
// 		  margin: 0 8px 10px 8px;
// 		  padding: 0;
// 		}
// 		/* Data table ------------------------------ */

// 		.purchase {
// 		  width: 100%;
// 		  margin: 0;
// 		  padding: 35px 0;
// 		  -premailer-width: 100%;
// 		  -premailer-cellpadding: 0;
// 		  -premailer-cellspacing: 0;
// 		}

// 		.purchase_content {
// 		  width: 100%;
// 		  margin: 0;
// 		  padding: 25px 0 0 0;
// 		  -premailer-width: 100%;
// 		  -premailer-cellpadding: 0;
// 		  -premailer-cellspacing: 0;
// 		}

// 		.purchase_item {
// 		  padding: 10px 0;
// 		  color: #51545E;
// 		  font-size: 15px;
// 		  line-height: 18px;
// 		}

// 		.purchase_heading {
// 		  padding-bottom: 8px;
// 		  border-bottom: 1px solid #EAEAEC;
// 		}

// 		.purchase_heading p {
// 		  margin: 0;
// 		  color: #85878E;
// 		  font-size: 12px;
// 		}

// 		.purchase_footer {
// 		  padding-top: 15px;
// 		  border-top: 1px solid #EAEAEC;
// 		}

// 		.purchase_total {
// 		  margin: 0;
// 		  text-align: right;
// 		  font-weight: bold;
// 		  color: #333333;
// 		}

// 		.purchase_total--label {
// 		  padding: 0 15px 0 0;
// 		}

// 		body {
// 		  background-color: #F2F4F6;
// 		  color: #51545E;
// 		}

// 		p {
// 		  color: #51545E;
// 		}

// 		.email-wrapper {
// 		  width: 100%;
// 		  margin: 0;
// 		  padding: 0;
// 		  -premailer-width: 100%;
// 		  -premailer-cellpadding: 0;
// 		  -premailer-cellspacing: 0;
// 		  background-color: #F2F4F6;
// 		}

// 		.email-content {
// 		  width: 100%;
// 		  margin: 0;
// 		  padding: 0;
// 		  -premailer-width: 100%;
// 		  -premailer-cellpadding: 0;
// 		  -premailer-cellspacing: 0;
// 		}
// 		/* Masthead ----------------------- */

// 		.email-masthead {
// 		  padding: 25px 0;
// 		  text-align: center;
// 		}

// 		.email-masthead_logo {
// 		  width: 94px;
// 		}

// 		.email-masthead_name {
// 		  font-size: 16px;
// 		  font-weight: bold;
// 		  color: #A8AAAF;
// 		  text-decoration: none;
// 		  text-shadow: 0 1px 0 white;
// 		}
// 		/* Body ------------------------------ */

// 		.email-body {
// 		  width: 100%;
// 		  margin: 0;
// 		  padding: 0;
// 		  -premailer-width: 100%;
// 		  -premailer-cellpadding: 0;
// 		  -premailer-cellspacing: 0;
// 		}

// 		.email-body_inner {
// 		  width: 570px;
// 		  margin: 0 auto;
// 		  padding: 0;
// 		  -premailer-width: 570px;
// 		  -premailer-cellpadding: 0;
// 		  -premailer-cellspacing: 0;
// 		  background-color: #FFFFFF;
// 		}

// 		.email-footer {
// 		  width: 570px;
// 		  margin: 0 auto;
// 		  padding: 0;
// 		  -premailer-width: 570px;
// 		  -premailer-cellpadding: 0;
// 		  -premailer-cellspacing: 0;
// 		  text-align: center;
// 		}

// 		.email-footer p {
// 		  color: #A8AAAF;
// 		}

// 		.body-action {
// 		  width: 100%;
// 		  margin: 30px auto;
// 		  padding: 0;
// 		  -premailer-width: 100%;
// 		  -premailer-cellpadding: 0;
// 		  -premailer-cellspacing: 0;
// 		  text-align: center;
// 		}

// 		.body-sub {
// 		  margin-top: 25px;
// 		  padding-top: 25px;
// 		  border-top: 1px solid #EAEAEC;
// 		}

// 		.content-cell {
// 		  padding: 45px;
// 		}
// 		/*Media Queries ------------------------------ */

// 		@media only screen and (max-width: 600px) {
// 		  .email-body_inner,
// 		  .email-footer {
// 			width: 100% !important;
// 		  }
// 		}

// 		@media (prefers-color-scheme: dark) {
// 		  body,
// 		  .email-body,
// 		  .email-body_inner,
// 		  .email-content,
// 		  .email-wrapper,
// 		  .email-masthead,
// 		  .email-footer {
// 			background-color: #333333 !important;
// 			color: #FFF !important;
// 		  }
// 		  p,
// 		  ul,
// 		  ol,
// 		  blockquote,
// 		  h1,
// 		  h2,
// 		  h3,
// 		  span,
// 		  .purchase_item {
// 			color: #FFF !important;
// 		  }
// 		  .attributes_content,
// 		  .discount {
// 			background-color: #222 !important;
// 		  }
// 		  .email-masthead_name {
// 			text-shadow: none !important;
// 		  }
// 		}

// 		:root {
// 		  color-scheme: light dark;
// 		  supported-color-schemes: light dark;
// 		}
// 		</style>
// 		<!--[if mso]>
// 		<style type="text/css">
// 		  .f-fallback  {
// 			font-family: Arial, sans-serif;
// 		  }
// 		</style>
// 	  <![endif]-->
// 	  </head>
// 	  <body>
// 		<span class="preheader">Use this link to reset your password. The link is only valid for 24 hours.</span>
// 		<table class="email-wrapper" width="100%" cellpadding="0" cellspacing="0" role="presentation">
// 		  <tr>
// 			<td align="center">
// 			  <table class="email-content" width="100%" cellpadding="0" cellspacing="0" role="presentation">
// 				<tr>
// 				  <td class="email-masthead">
// 					<a href="https://www.themonkeys.life" class="f-fallback email-masthead_name">
// 					The Monkeys
// 				  </a>
// 				  </td>
// 				</tr>
// 				<!-- Email Body -->
// 				<tr>
// 				  <td class="email-body" width="570" cellpadding="0" cellspacing="0">
// 					<table class="email-body_inner" align="center" width="570" cellpadding="0" cellspacing="0" role="presentation">
// 					  <!-- Body content -->
// 					  <tr>
// 						<td class="content-cell">
// 						  <div class="f-fallback">
// 							<h1>Hi ` + firstName + ` ` + LastName + `,</h1>
// 							<p>You recently requested to reset your password for your The Monkeys account. Use the button below to reset it. <strong>This password reset link is only valid for the next 5 minutes.</strong></p>
// 							<!-- Action -->
// 							<table class="body-action" align="center" width="100%" cellpadding="0" cellspacing="0" role="presentation">
// 							  <tr>
// 								<td align="center">

// 								  <table width="100%" border="0" cellspacing="0" cellpadding="0" role="presentation">
// 									<tr>
// 									  <td align="center">
// 										<a href="` + Address + `/auth/reset-password?user=` + username + `&evpw=` + secret + `" class="f-fallback button button--green" target="_blank">Reset your password</a>
// 									  </td>
// 									</tr>
// 								  </table>
// 								</td>
// 							  </tr>
// 							</table>
// 							<p>If you did not request a password reset, please ignore this email or <a href="{{https://www.themonkeys.life/contact}}">contact support</a> if you have questions.</p>
// 							<p>Thanks,
// 							  <br>The Monkeys team</p>
// 							<!-- Sub copy -->

// 						  </div>
// 						</td>
// 					  </tr>
// 					</table>
// 				  </td>
// 				</tr>
// 			  </table>
// 			</td>
// 		  </tr>
// 		</table>
// 	  </body>
// 	</html>`

// }

func EmailVerificationHTML(firstName, lastName, username, secret string) string {
	return `<!DOCTYPE html>
<html lang="en">
	<head>
		<meta charset="UTF-8" />
		<meta name="viewport" content="width=device-width, initial-scale=1.0" />
		<title>Welcome to Monkeys</title>
		<style>
			body {
				font-family: Arial, sans-serif;
				background-color: #fff4ed;
				margin: 0;
				padding: 20px;
				display: flex;
				flex-direction: column;
				justify-content: center;
				align-items: center;
				min-height: 100vh;
			}

			h1 {
				color: #101010;
				margin-bottom: 20px;
				text-align: center;
				font-family: serif;
				font-size: 36px;
			}

			p {
				color: #000;
				line-height: 1.6;
				margin-bottom: 20px;
				font-weight: 500;
				font-size: 16px;
			}

			a {
				text-decoration: none;
				display: inline-block;
			}

			.btn {
				background-color: #ff462e;
				color: #f2f2f3;
				border: none;
				padding: 15px 30px;
				text-transform: uppercase;
				font-weight: 600;
				font-size: 16px;
				border-radius: 5px;
				cursor: pointer;
			}

			.btn:hover {
				background-color: #f7381f;
			}

			.footer {
				margin-top: 40px;
				display: flex;
				flex-direction: column;
				align-items: center;
			}

			.footer_icon {
				opacity: 0.75;
			}

			.footer_icon:hover {
				opacity: 1;
			}
		</style>
	</head>

	<body>
		<main class="container">
			<svg
				width="127"
				height="32"
				viewBox="0 0 127 32"
				fill="none"
				xmlns="http://www.w3.org/2000/svg"
			>
				<path
					d="M39.2518 15.3296C38.5395 15.2382 38.4262 15.6611 38.4218 16.2842C38.4166 17.0562 37.8897 17.2174 37.2598 17.2097C36.5739 17.2005 36.2215 16.849 36.1935 16.1422C36.1876 15.9842 36.1972 15.8207 36.2281 15.6665C36.387 14.8845 35.1169 14.3496 35.8454 13.5039C36.557 12.6774 37.1663 13.8776 37.9471 13.675C39.3416 13.3136 39.48 14.2414 39.215 15.3748C39.2136 15.3764 39.2504 15.3303 39.2504 15.3303L39.2518 15.3296Z"
					fill="#FF462E"
				/>
				<path
					d="M31.8056 15.4503C31.2308 14.0408 32.5683 14.3908 32.935 13.846C33.1341 13.5503 33.5374 13.79 33.5046 14.0581C33.4189 14.7572 33.89 15.5736 33.4509 16.1115C32.5451 17.2209 32.4371 15.3193 31.7631 15.4003C31.7646 15.4012 31.8064 15.4512 31.8064 15.4512L31.8056 15.4503Z"
					fill="#FF462E"
				/>
				<path
					d="M40.3611 21.3305C41.2157 20.1011 40.9198 18.8717 40.2308 17.736C39.7557 16.9522 39.4357 16.1643 39.3899 15.2578L39.3497 15.3071C39.9888 14.2521 40.1553 13.1607 39.2275 12.2276C38.1888 11.1822 36.4425 11.2194 34.9456 12.1848C35.4408 10.6785 36.6966 9.814 38.4822 9.98029C39.9293 10.1151 41.2889 10.5268 41.2438 12.456C41.2382 12.6941 41.4312 13.109 41.6064 13.1542C44.4854 13.9058 44.6945 16.4057 45.0699 18.6643C45.3352 20.2561 45.1664 21.9173 45.2516 23.5447C45.3031 24.5238 44.8247 24.8919 43.9556 24.8241C41.6804 24.6473 39.3971 25.4441 37.0913 24.4488C36.1869 24.0581 34.8981 24.5472 33.787 24.6659C33.2428 24.724 32.6944 24.9387 32.1582 24.3915C35.1674 23.8926 32.5923 21.9343 33.2524 20.6242C33.5668 22.4065 34.6063 22.7972 36.1097 22.2895C36.2665 22.2362 36.4651 22.1619 36.6009 22.2128C38.0794 22.7698 39.4003 22.7221 40.4013 21.295C41.2487 21.7478 41.9963 21.7785 42.0743 20.5943C42.1636 19.2406 42.0952 17.8764 42.0952 16.0312C41.206 17.6399 41.2736 18.9395 41.1578 20.1931C41.1063 20.7469 41.2462 21.3854 40.3619 21.3305H40.3611Z"
					fill="#FF462E"
				/>
				<path
					d="M31.544 15.5251C31.4779 16.1724 31.6555 16.7817 32.0456 17.2547C32.8624 18.2452 32.7995 19.5181 32.1236 20.1947C30.9844 21.3353 31.1571 22.5887 31.15 23.8972C31.146 24.5949 31.177 25.3161 30.3905 25.5976C29.6588 25.8588 28.9861 25.6463 28.3763 25.149C26.5078 23.6246 25.7626 21.5048 25.5525 19.2147C24.8097 11.1191 30.3984 6.12343 38.6104 7.07584C47.0404 8.05422 50.5059 16.445 47.8262 23.4502C47.5754 24.1057 47.2052 24.6119 46.632 24.9421C46.2833 24.8002 46.3805 24.5884 46.4816 24.3921C49.2433 18.9989 47.9759 13.4263 43.1148 9.60209C39.1812 6.50715 32.8194 6.9647 28.8524 10.6275C25.4601 13.7598 25.1783 20.7601 28.3333 24.0895C28.7799 24.5608 29.2783 25.2252 30.0187 24.9259C30.7065 24.6476 30.5433 23.8786 30.5425 23.2709C30.5409 22.1766 30.5131 21.2428 29.6986 20.2036C28.6685 18.8893 28.4129 17.077 29.5028 15.448C29.6859 15.1738 29.8849 14.9661 29.9335 14.5832C30.1779 12.6573 30.65 12.3409 32.6307 12.6508C30.5831 14.1508 30.5258 14.3122 31.5886 15.5737L31.544 15.5275V15.5251Z"
					fill="#FF462E"
				/>
				<path
					d="M5.37725 10.8279L3.85897 24.46H0L3.03656 0.939941L12.1146 16.1399L21.2243 0.939941L24.2609 24.46H20.4019L18.852 10.8279L12.1146 22.3799L5.37725 10.8279Z"
					fill="#101010"
				/>
				<path
					d="M60.2607 15.9999C60.2607 14.9119 60.0393 14.0906 59.5964 13.5359C59.1536 12.9813 58.4683 12.7039 57.5404 12.7039C56.9078 12.7039 56.349 12.8426 55.864 13.1199C55.379 13.3759 54.9994 13.7599 54.7253 14.2719C54.4512 14.7626 54.3141 15.3386 54.3141 15.9999V24.9599H50.8979V10.2399H54.3141V12.5119C54.778 11.6373 55.3684 10.9866 56.0854 10.5599C56.8235 10.1333 57.6986 9.91992 58.7108 9.91992C60.3556 9.91992 61.6103 10.4319 62.4749 11.4559C63.3394 12.4586 63.7717 13.8453 63.7717 15.6159V24.9599H60.2607V15.9999Z"
					fill="#101010"
				/>
				<path
					d="M68.0698 0H71.4227V24.96H68.0698V0ZM76.7683 10.24H80.9436L74.6807 16.32L81.5762 24.96H77.4326L70.4738 16.32L76.7683 10.24Z"
					fill="#101010"
				/>
				<path
					d="M90.338 25.2799C88.8619 25.2799 87.5545 24.9599 86.4158 24.3199C85.2982 23.6799 84.4336 22.7839 83.8221 21.6319C83.2106 20.4799 82.9048 19.1359 82.9048 17.5999C82.9048 16.0426 83.2106 14.6879 83.8221 13.5359C84.4547 12.3839 85.3404 11.4986 86.4791 10.8799C87.6178 10.2399 88.9568 9.91992 90.4962 9.91992C92.0356 9.91992 93.343 10.2186 94.4184 10.8159C95.4939 11.4133 96.3163 12.2773 96.8856 13.4079C97.4761 14.5173 97.7713 15.8613 97.7713 17.4399C97.7713 17.6106 97.7608 17.7919 97.7397 17.9839C97.7397 18.1759 97.7291 18.3146 97.708 18.3999H84.9292V16.0319H94.798L93.7542 17.5039C93.8174 17.3759 93.8807 17.2053 93.944 16.9919C94.0283 16.7573 94.0705 16.5653 94.0705 16.4159C94.0705 15.6266 93.9123 14.9439 93.596 14.3679C93.3008 13.7919 92.8791 13.3439 92.3308 13.0239C91.8036 12.7039 91.1815 12.5439 90.4646 12.5439C89.6 12.5439 88.8619 12.7359 88.2504 13.1199C87.6389 13.5039 87.175 14.0586 86.8586 14.7839C86.5423 15.5093 86.3736 16.4053 86.3526 17.4719C86.3526 18.5386 86.5107 19.4453 86.827 20.1919C87.1433 20.9173 87.6072 21.4719 88.2188 21.8559C88.8514 22.2399 89.6105 22.4319 90.4962 22.4319C91.424 22.4319 92.2359 22.2399 92.9318 21.8559C93.6277 21.4719 94.2076 20.8853 94.6715 20.0959L97.6131 21.3119C96.854 22.6346 95.8734 23.6266 94.6715 24.2879C93.4695 24.9493 92.025 25.2799 90.338 25.2799Z"
					fill="#101010"
				/>
				<path
					d="M113.936 10.24L104.447 32H100.809L104.605 23.232L98.7534 10.24H102.676L107.325 21.888L105.712 21.792L110.267 10.24H113.936Z"
					fill="#101010"
				/>
				<path
					d="M116.801 20.1279C117.265 20.6399 117.729 21.0879 118.193 21.4719C118.678 21.8346 119.163 22.1119 119.648 22.3039C120.133 22.4746 120.618 22.5599 121.103 22.5599C121.714 22.5599 122.189 22.4213 122.526 22.1439C122.885 21.8666 123.064 21.4826 123.064 20.9919C123.064 20.5653 122.927 20.2026 122.653 19.9039C122.378 19.5839 121.999 19.3173 121.514 19.1039C121.029 18.8693 120.459 18.6346 119.806 18.3999C119.152 18.1439 118.498 17.8453 117.845 17.5039C117.212 17.1413 116.685 16.6719 116.263 16.0959C115.862 15.5199 115.662 14.7946 115.662 13.9199C115.662 13.0239 115.884 12.2773 116.326 11.6799C116.79 11.0826 117.402 10.6453 118.161 10.3679C118.941 10.0693 119.774 9.91992 120.66 9.91992C121.461 9.91992 122.21 10.0373 122.906 10.2719C123.623 10.5066 124.266 10.8159 124.835 11.1999C125.404 11.5839 125.889 12.0213 126.29 12.5119L124.392 14.5599C123.907 13.9839 123.327 13.5146 122.653 13.1519C121.978 12.7893 121.282 12.6079 120.565 12.6079C120.08 12.6079 119.679 12.7146 119.363 12.9279C119.047 13.1413 118.888 13.4506 118.888 13.8559C118.888 14.1973 119.026 14.5066 119.3 14.7839C119.595 15.0399 119.974 15.2746 120.438 15.4879C120.923 15.7013 121.461 15.9253 122.052 16.1599C122.853 16.4799 123.591 16.8319 124.266 17.2159C124.941 17.5999 125.478 18.0693 125.879 18.6239C126.301 19.1786 126.512 19.9039 126.512 20.7999C126.512 22.1866 126.037 23.3066 125.088 24.1599C124.16 24.9919 122.895 25.4079 121.292 25.4079C120.301 25.4079 119.384 25.2586 118.541 24.9599C117.718 24.6399 116.991 24.2346 116.358 23.7439C115.746 23.2319 115.23 22.6986 114.808 22.1439L116.801 20.1279Z"
					fill="#101010"
				/>
			</svg>

			<h1 style="margin-bottom: 100px; cursor: default">
				Welcome to <span style="color: #ff462e">Monkeys</span>
			</h1>

      <p>Hello ` + firstName + ` ` + lastName + `,</p>

			<p>
				Thanks for joining <b>The Monkeys</b>. We're excited to have you
				on board.<br />
				To complete your registration and explore all that The Monkeys
				has to offer, please verify your email address. This ensures
				your account's security and keeps you informed about important
				updates.
			</p>

			<p>
				It's easy! Just click the verify button to confirm your email:
			</p>

			<div style="margin: 20px 0; display: flex; justify-content: center">
				<a href="` + Address + `/auth/verify-email?user=` + username + `&evpw=` + secret + `" target="_blank"><button class="btn">Verify Email Address</button></a>
			</div>

			<p>
				<b>Alternatively</b>, you can copy and paste the link below into
				your web browser:
			</p>

      		<a href="` + Address + `/auth/verify-email?user=` + username + `&evpw=` + secret + `" target="_blank">` + Address + `/auth/verify-email?user=` + username + `&evpw=` + secret + `</a>

			<p style="color: #ed3232">
				This link will expire in 24 hours for your security. If you
				don't verify your email within that time, you can request a new
				link anytime.
			</p>

			<p>
				Once you verify your email, you'll be ready to dive in and start
				using <span style="font-weight: bold">The Monkeys</span>. If you
				have any trouble verifying your email, please feel free to
				contact our support team at <b>mail.themonkeys.life@gmail.com</b>. We're happy
				to help.
			</p>

			<p>Welcome to the community,</p>

			<p>Thanks,<br />The Monkeys Team</p>

			<footer class="footer">
				<div style="display: flex; gap: 10px">
					<a
						href="https://github.com/the-monkeys"
						target="_blank"
						aria-label="GitHub"
					>
						<div
							style="height: 24px; width: 24px"
							class="footer_icon"
						>
							<svg
								xmlns="http://www.w3.org/2000/svg"
								viewBox="0 0 24 24"
								fill="#101010"
							>
								<path
									d="M12.001 2C6.47598 2 2.00098 6.475 2.00098 12C2.00098 16.425 4.86348 20.1625 8.83848 21.4875C9.33848 21.575 9.52598 21.275 9.52598 21.0125C9.52598 20.775 9.51348 19.9875 9.51348 19.15C7.00098 19.6125 6.35098 18.5375 6.15098 17.975C6.03848 17.6875 5.55098 16.8 5.12598 16.5625C4.77598 16.375 4.27598 15.9125 5.11348 15.9C5.90098 15.8875 6.46348 16.625 6.65098 16.925C7.55098 18.4375 8.98848 18.0125 9.56348 17.75C9.65098 17.1 9.91348 16.6625 10.201 16.4125C7.97598 16.1625 5.65098 15.3 5.65098 11.475C5.65098 10.3875 6.03848 9.4875 6.67598 8.7875C6.57598 8.5375 6.22598 7.5125 6.77598 6.1375C6.77598 6.1375 7.61348 5.875 9.52598 7.1625C10.326 6.9375 11.176 6.825 12.026 6.825C12.876 6.825 13.726 6.9375 14.526 7.1625C16.4385 5.8625 17.276 6.1375 17.276 6.1375C17.826 7.5125 17.476 8.5375 17.376 8.7875C18.0135 9.4875 18.401 10.375 18.401 11.475C18.401 15.3125 16.0635 16.1625 13.8385 16.4125C14.201 16.725 14.5135 17.325 14.5135 18.2625C14.5135 19.6 14.501 20.675 14.501 21.0125C14.501 21.275 14.6885 21.5875 15.1885 21.4875C19.259 20.1133 21.9999 16.2963 22.001 12C22.001 6.475 17.526 2 12.001 2Z"
								></path>
							</svg>
						</div>
					</a>

					<a
						href="https://x.com/TheMonkeysLife"
						target="_blank"
						aria-label="Twitter"
					>
						<div
							style="height: 24px; width: 24px"
							class="footer_icon"
						>
							<svg
								xmlns="http://www.w3.org/2000/svg"
								viewBox="0 0 24 24"
								fill="#101010"
							>
								<path
									d="M8 2H1L9.26086 13.0145L1.44995 21.9999H4.09998L10.4883 14.651L16 22H23L14.3917 10.5223L21.8001 2H19.1501L13.1643 8.88578L8 2ZM17 20L5 4H7L19 20H17Z"
								></path>
							</svg>
						</div>
					</a>

					<a
						href="https://www.instagram.com/themonkeys"
						target="_blank"
						aria-label="Instagram"
					>
						<div
							style="height: 24px; width: 24px"
							class="footer_icon"
						>
							<svg
								xmlns="http://www.w3.org/2000/svg"
								viewBox="0 0 24 24"
								fill="#101010"
							>
								<path
									d="M13.0281 2.00073C14.1535 2.00259 14.7238 2.00855 15.2166 2.02322L15.4107 2.02956C15.6349 2.03753 15.8561 2.04753 16.1228 2.06003C17.1869 2.1092 17.9128 2.27753 18.5503 2.52503C19.2094 2.7792 19.7661 3.12253 20.3219 3.67837C20.8769 4.2342 21.2203 4.79253 21.4753 5.45003C21.7219 6.0867 21.8903 6.81337 21.9403 7.87753C21.9522 8.1442 21.9618 8.3654 21.9697 8.58964L21.976 8.78373C21.9906 9.27647 21.9973 9.84686 21.9994 10.9723L22.0002 11.7179C22.0003 11.809 22.0003 11.903 22.0003 12L22.0002 12.2821L21.9996 13.0278C21.9977 14.1532 21.9918 14.7236 21.9771 15.2163L21.9707 15.4104C21.9628 15.6347 21.9528 15.8559 21.9403 16.1225C21.8911 17.1867 21.7219 17.9125 21.4753 18.55C21.2211 19.2092 20.8769 19.7659 20.3219 20.3217C19.7661 20.8767 19.2069 21.22 18.5503 21.475C17.9128 21.7217 17.1869 21.89 16.1228 21.94C15.8561 21.9519 15.6349 21.9616 15.4107 21.9694L15.2166 21.9757C14.7238 21.9904 14.1535 21.997 13.0281 21.9992L12.2824 22C12.1913 22 12.0973 22 12.0003 22L11.7182 22L10.9725 21.9993C9.8471 21.9975 9.27672 21.9915 8.78397 21.9768L8.58989 21.9705C8.36564 21.9625 8.14444 21.9525 7.87778 21.94C6.81361 21.8909 6.08861 21.7217 5.45028 21.475C4.79194 21.2209 4.23444 20.8767 3.67861 20.3217C3.12278 19.7659 2.78028 19.2067 2.52528 18.55C2.27778 17.9125 2.11028 17.1867 2.06028 16.1225C2.0484 15.8559 2.03871 15.6347 2.03086 15.4104L2.02457 15.2163C2.00994 14.7236 2.00327 14.1532 2.00111 13.0278L2.00098 10.9723C2.00284 9.84686 2.00879 9.27647 2.02346 8.78373L2.02981 8.58964C2.03778 8.3654 2.04778 8.1442 2.06028 7.87753C2.10944 6.81253 2.27778 6.08753 2.52528 5.45003C2.77944 4.7917 3.12278 4.2342 3.67861 3.67837C4.23444 3.12253 4.79278 2.78003 5.45028 2.52503C6.08778 2.27753 6.81278 2.11003 7.87778 2.06003C8.14444 2.04816 8.36564 2.03847 8.58989 2.03062L8.78397 2.02433C9.27672 2.00969 9.8471 2.00302 10.9725 2.00086L13.0281 2.00073ZM12.0003 7.00003C9.23738 7.00003 7.00028 9.23956 7.00028 12C7.00028 14.7629 9.23981 17 12.0003 17C14.7632 17 17.0003 14.7605 17.0003 12C17.0003 9.23713 14.7607 7.00003 12.0003 7.00003ZM12.0003 9.00003C13.6572 9.00003 15.0003 10.3427 15.0003 12C15.0003 13.6569 13.6576 15 12.0003 15C10.3434 15 9.00028 13.6574 9.00028 12C9.00028 10.3431 10.3429 9.00003 12.0003 9.00003ZM17.2503 5.50003C16.561 5.50003 16.0003 6.05994 16.0003 6.74918C16.0003 7.43843 16.5602 7.9992 17.2503 7.9992C17.9395 7.9992 18.5003 7.4393 18.5003 6.74918C18.5003 6.05994 17.9386 5.49917 17.2503 5.50003Z"
								></path>
							</svg>
						</div>
					</a>
				</div>

				<p style="font-size: 14px; opacity: 0.75">
					Monkeys, 2024, All Rights Reserved
				</p>
			</footer>
		</main>
	</body>
</html>`
}

// func EmailVerificationHTMLa(username, secret string) string {
// 	return `<!DOCTYPE html>
// 	<html>
// 	<head>

// 	  <meta charset="utf-8">
// 	  <meta http-equiv="x-ua-compatible" content="ie=edge">
// 	  <title>The Monkeys</title>
// 	  <meta name="viewport" content="width=device-width, initial-scale=1">
// 	  <style type="text/css">

// 	  </style>

// 	</head>
// 	<body style="background-color: #e9ecef;">

// 		<div style="display: none; font-size: 1px; color: #fefefe; line-height: 1px; font-family: 'Lato', Helvetica, Arial, sans-serif; max-height: 0px; max-width: 0px; opacity: 0; overflow: hidden;"> We're thrilled to have you here! Get ready to dive into your new account. </div>
// 		<table border="0" cellpadding="0" cellspacing="0" width="100%">
// 			<!-- LOGO -->
// 			<tr>
// 				<td bgcolor="#FFA73B" align="center">
// 					<table border="0" cellpadding="0" cellspacing="0" width="100%" style="max-width: 600px;">
// 						<tr>
// 							<td align="center" valign="top" style="padding: 40px 10px 40px 10px;"> </td>
// 						</tr>
// 					</table>
// 				</td>
// 			</tr>
// 			<tr>
// 				<td bgcolor="#FFA73B" align="center" style="padding: 0px 10px 0px 10px;">
// 					<table border="0" cellpadding="0" cellspacing="0" width="100%" style="max-width: 600px;">
// 						<tr>
// 							<td bgcolor="#ffffff" align="center" valign="top" style="padding: 40px 20px 20px 20px; border-radius: 4px 4px 0px 0px; color: #111111; font-family: 'Lato', Helvetica, Arial, sans-serif; font-size: 48px; font-weight: 400; letter-spacing: 4px; line-height: 48px;">
// 								<h1 style="font-size: 48px; font-weight: 400; margin: 2;">The Monkeys</h1> <img src=" https://img.icons8.com/clouds/100/000000/handshake.png" width="125" height="120" style="display: block; border: 0px;" />
// 							</td>
// 						</tr>
// 					</table>
// 				</td>
// 			</tr>
// 			<tr>
// 				<td bgcolor="#f4f4f4" align="center" style="padding: 0px 10px 0px 10px;">
// 					<table border="0" cellpadding="0" cellspacing="0" width="100%" style="max-width: 600px;">
// 						<tr>
// 							<td bgcolor="#ffffff" align="left" style="padding: 20px 30px 40px 30px; color: #666666; font-family: 'Lato', Helvetica, Arial, sans-serif; font-size: 18px; font-weight: 400; line-height: 25px;">
// 								<p style="margin: 0;">We're excited to have you get started. First, you need to confirm your account. Just press the button below.</p>
// 							</td>
// 						</tr>
// 						<tr>
// 							<td bgcolor="#ffffff" align="left">
// 								<table width="100%" border="0" cellspacing="0" cellpadding="0">
// 									<tr>
// 										<td bgcolor="#ffffff" align="center" style="padding: 20px 30px 60px 30px;">
// 											<table border="0" cellspacing="0" cellpadding="0">
// 												<tr>
// 													<td align="center" style="border-radius: 3px;" bgcolor="#FFA73B"><a href="` + Address + `/auth/verify-email?user=` + username + `&evpw=` + secret + `" target="_blank" style="font-size: 20px; font-family: Helvetica, Arial, sans-serif; color: #ffffff; text-decoration: none; color: #ffffff; text-decoration: none; padding: 15px 25px; border-radius: 2px; border: 1px solid #FFA73B; display: inline-block;">Confirm Account</a></td>
// 												</tr>
// 											</table>
// 										</td>
// 									</tr>
// 								</table>
// 							</td>
// 						</tr> <!-- COPY -->
// 						<tr>
// 							<td bgcolor="#ffffff" align="left" style="padding: 0px 30px 0px 30px; color: #666666; font-family: 'Lato', Helvetica, Arial, sans-serif; font-size: 18px; font-weight: 400; line-height: 25px;">
// 								<p style="margin: 0;">If that doesn't work, copy and paste the following link in your browser:</p>
// 							</td>
// 						</tr> <!-- COPY -->
// 						<tr>
// 							<td bgcolor="#ffffff" align="left" style="padding: 20px 30px 20px 30px; color: #666666; font-family: 'Lato', Helvetica, Arial, sans-serif; font-size: 18px; font-weight: 400; line-height: 25px;">
// 								<p style="margin: 0;"><a href="` + Address + `/auth/verify-email?user=` + username + `&evpw=` + secret + `" target="_blank" style="color: #FFA73B;">` + Address + `/auth/verify-email?user=` + username + `&evpw=` + secret + `</a></p>
// 							</td>
// 						</tr>
// 						<tr>
// 							<td bgcolor="#ffffff" align="left" style="padding: 0px 30px 20px 30px; color: #666666; font-family: 'Lato', Helvetica, Arial, sans-serif; font-size: 18px; font-weight: 400; line-height: 25px;">
// 								<p style="margin: 0;">If you have any questions, just reply to this email—we're always happy to help out.</p>
// 							</td>
// 						</tr>
// 						<tr>
// 							<td bgcolor="#ffffff" align="left" style="padding: 0px 30px 40px 30px; border-radius: 0px 0px 4px 4px; color: #666666; font-family: 'Lato', Helvetica, Arial, sans-serif; font-size: 18px; font-weight: 400; line-height: 25px;">
// 								<p style="margin: 0;">Cheers,<br>The Monkeys Team</p>
// 							</td>
// 						</tr>
// 					</table>
// 				</td>
// 			</tr>
// 			<tr>
// 				<td bgcolor="#f4f4f4" align="center" style="padding: 30px 10px 0px 10px;">
// 					<table border="0" cellpadding="0" cellspacing="0" width="100%" style="max-width: 600px;">
// 						<tr>
// 							<td bgcolor="#FFECD1" align="center" style="padding: 30px 30px 30px 30px; border-radius: 4px 4px 4px 4px; color: #666666; font-family: 'Lato', Helvetica, Arial, sans-serif; font-size: 18px; font-weight: 400; line-height: 25px;">
// 								<h2 style="font-size: 20px; font-weight: 400; color: #111111; margin: 0;">Need more help?</h2>
// 								<p style="margin: 0;"><a href="themonkeys.life" target="_blank" style="color: #FFA73B;">We&rsquo;re here to help you out</a></p>
// 							</td>
// 						</tr>
// 					</table>
// 				</td>
// 			</tr>
// 			<tr>
// 				<td bgcolor="#f4f4f4" align="center" style="padding: 0px 10px 0px 10px;">
// 					<table border="0" cellpadding="0" cellspacing="0" width="100%" style="max-width: 600px;">
// 						<tr>
// 							<td bgcolor="#f4f4f4" align="left" style="padding: 0px 30px 30px 30px; color: #666666; font-family: 'Lato', Helvetica, Arial, sans-serif; font-size: 14px; font-weight: 400; line-height: 18px;"> <br>
// 								<p style="margin: 0;">If these emails get annoying, please feel free to <a href="#" target="_blank" style="color: #111111; font-weight: 700;">unsubscribe</a>.</p>
// 							</td>
// 						</tr>
// 					</table>
// 				</td>
// 			</tr>
// 		</table>

// 	</body>
// 	</html>`
// }
